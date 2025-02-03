# Copyright 2023 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import os
import sys
import traceback
import time
import json
import nuclio_sdk
import nuclio_sdk.helpers
import nuclio_sdk.json_encoder
import nuclio_sdk.logger
import socket
import asyncio
import selectors

from wrapper_common import (
    WrapperFatalException,
    AbstractWrapper,
    create_logger,
    get_parser_with_common_args)


class AsyncWrapper(AbstractWrapper):
    def __init__(self,
                 logger,
                 loop,
                 handler,
                 serving_address,
                 control_socket_path,
                 platform_kind,
                 namespace=None,
                 worker_id=None,
                 trigger_kind=None,
                 trigger_name=None,
                 decode_event_strings=True,
                 max_connections=None):
        super().__init__(
            logger,
            loop,
            handler,
            control_socket_path,
            platform_kind,
            namespace,
            worker_id,
            trigger_kind,
            trigger_name,
            decode_event_strings,
        )
        # Validate that the handler is an async function
        if not asyncio.iscoroutinefunction(self._entrypoint):
            raise WrapperFatalException(f"The provided handler '{self._entrypoint.__name__}' "
                                        f"must be an async function (async def).")
        split_address = serving_address.split(":")
        self._host = split_address[0]
        self._port = int(split_address[1])

        # Max file descriptors allowed by the OS
        self._max_connections = max_connections if max_connections else os.sysconf("SC_OPEN_MAX")
        self.selector = selectors.DefaultSelector()
        self.connections = {}  # Track active sockets
        self.tasks = {}  # Track active asyncio tasks
        self.shutdown_event = asyncio.Event()

    async def start(self):
        """Start the server."""
        # Create server socket
        server_sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        server_sock.setsockopt(
            # Level: Applies the option at the socket level
            socket.SOL_SOCKET,
            # Option: Allows reuse of the address/port
            socket.SO_REUSEADDR,
            # Value: Enables the option (0 would disable it)
            1,
        )
        server_sock.bind((self._host, self._port))
        server_sock.listen(self._max_connections)
        server_sock.setblocking(False)

        self._logger.info(f"Python wrapper server started on {self._host}:{self._port}, "
                          f"max_connections: {self._max_connections}")

        # Register server socket with default os selector (epoll for Linux)
        self.selector.register(server_sock, selectors.EVENT_READ)
        self.connections[server_sock.fileno()] = server_sock
        try:
            while True:
                events = await self.select_events()
                # Now awaiting the select operation
                for key, event in events:
                    sock = key.fileobj
                    if sock == server_sock:
                        # Accept new connection
                        client_sock, addr = server_sock.accept()
                        self._logger.debug(f"Accepted connection from {addr}")
                        client_sock.setblocking(False)
                        self.connections[client_sock.fileno()] = client_sock
                        task = self._loop.create_task(self._process_connection(client_sock))
                        self.tasks[client_sock.fileno()] = task
                    else:
                        self._logger.error(f"Unexpected event: {event}")
                await asyncio.sleep(0.2)
        except KeyboardInterrupt:
            self._logger.info("Shutting down server")
        finally:
            self._logger.info("Closing all connections")
            for sock in self.connections.values():
                sock.close()

    async def select_events(self, timeout=0.1):
        # Run the selector.select in the default executor (this is a blocking operation)
        events = await self._loop.run_in_executor(None, self.selector.select, timeout)
        return events

    async def initialize(self):

        # call init_context
        await self._initialize_context()

        # register to the SIGUSR1 and SIGUSR2 signals, used to signal termination/draining respectively
        self._register_to_signal()

        await self._send_data_on_control_socket({
            'kind': 'wrapperInitialized',
            'attributes': {'ready': 'true'}
        })

    async def _process_connection(self, sock):
        """Process all events for a single connection."""
        # signal start
        self._logger.debug("Signalling connection processing start")
        await self._write_packet_to_processor(sock, 's')
        self._logger.debug(f"Event processing started for socket")
        try:
            while True:
                # resolve event message length
                event_message_length = await self._resolve_event_message_length(sock)
                # Resolve event message
                event = await self._resolve_event(sock, event_message_length)

                # Handle the event
                if not self._discard_events:
                    try:
                        await self._handle_event(event, sock)
                    except BaseException as exc:
                        await self._on_handle_event_error(exc, sock)
                else:
                    self._logger.debug("Event discarded")

                # Release event reference
                del event

        except (ConnectionResetError, asyncio.IncompleteReadError):
            self._logger.info("Client disconnected")
        except WrapperFatalException as exc:
            await self._on_serving_error(exc, sock)

            # explode, unrecoverable exception
            await self._shutdown(error_code=1)

        except UnicodeDecodeError as exc:

            # reset unpacker to avoid consecutive errors
            # this may happen when msgpack fails to decode a non-utf8 events
            self._unpacker = self._resolve_unpacker()
            await self._on_serving_error(exc, sock)

        except asyncio.CancelledError:
            self._logger.debug('Connection processing was cancelled by a signal')
        except Exception as exc:
            await self._on_serving_error(exc, sock)
        finally:
            if self._is_drain_needed:
                result = self._call_drain_handler()
                if asyncio.iscoroutine(result):
                    await result

            if self._is_termination_needed:
                result = self._call_termination_handler()
                if asyncio.iscoroutine(result):
                    await result
            self._cleanup_connection(sock, cancel_task=False)

    async def _handle_event(self, event, sock):
        # take call time
        start_time = time.time()

        # call the entrypoint
        entrypoint_output = await self._entrypoint(self._context, event)

        # measure duration, set to minimum float in case execution was too fast
        duration = time.time() - start_time or sys.float_info.min

        await self._write_packet_to_processor(sock, 'm' + json.dumps({'duration': duration}))

        # try to json encode the response
        encoded_response = self._encode_entrypoint_output(entrypoint_output)

        # write response to the socket
        await self._write_packet_to_processor(sock, 'r' + encoded_response)

    def _cleanup_connection(self, sock, cancel_task=True):
        """Cleanup resources for a disconnected client."""
        fileno = sock.fileno()
        if fileno in self.connections:
            del self.connections[fileno]
            self._logger.info(f"Connection {fileno} removed from tracking")
        sock.close()

        # Cancel and remove the associated task
        # do not cancel if calling from the task itself
        if cancel_task and fileno in self.tasks:
            task = self.tasks.pop(fileno)
            if not task.done():
                task.cancel()
            self._logger.info(f"Task for connection {fileno} cancelled and removed")

    async def _shutdown(self, error_code=0):
        """Shutdown the server and cleanup resources."""
        self._logger.info("Shutting down...")
        try:
            await self._stop_processing()
        finally:
            super()._shutdown()

    async def _stop_processing(self):
        self.shutdown_event.set()

        # Cancel all active tasks
        for fileno, task in list(self.tasks.items()):
            self._logger.info(f"Cancelling task for connection {fileno}")
            task.cancel()
        # Close all active sockets
        for fileno, sock in list(self.connections.items()):
            self._logger.info(f"Closing connection {fileno}")
            sock.close()
        self.selector.close()


def parse_args():
    parser = get_parser_with_common_args()
    return parser.parse_args()


def run_wrapper():
    # parse arguments
    args = parse_args()

    # create a logger instance. note: there are no outputters until socket is created
    root_logger = create_logger(args.log_level)

    # add a logger output that is in a JSON format. we'll remove it once we have a socket output. this
    # way all output goes to stdout until a socket is available and then switches exclusively to socket
    root_logger.set_handler('default', sys.stdout, nuclio_sdk.logger.JSONFormatter())

    # bind worker_id to the logger
    root_logger.bind(worker_id=args.worker_id)

    loop = asyncio.get_event_loop()

    try:

        # create a new wrapper
        wrapper_instance = AsyncWrapper(root_logger,
                                        loop,
                                        args.handler,
                                        # TODO: rename it to serving_address or smth like it
                                        args.event_socket_path,
                                        args.control_socket_path,
                                        args.platform_kind,
                                        args.namespace,
                                        args.worker_id,
                                        args.trigger_kind,
                                        args.trigger_name,
                                        args.decode_event_strings)

    except BaseException as exc:
        root_logger.error_with('Caught unhandled exception while initializing',
                               err=str(exc),
                               traceback=traceback.format_exc())

        raise SystemExit(1)

    try:
        loop.run_until_complete(wrapper_instance.initialize())
        loop.run_until_complete(wrapper_instance.start())
    finally:

        # finalize all scheduled asynchronous generators reliably
        loop.run_until_complete(loop.shutdown_asyncgens())
        loop.close()


if __name__ == '__main__':
    # run the wrapper
    run_wrapper()
