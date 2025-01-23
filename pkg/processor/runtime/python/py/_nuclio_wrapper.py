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

import asyncio
import sys
import traceback

import nuclio_sdk
import nuclio_sdk.helpers
import nuclio_sdk.json_encoder
import nuclio_sdk.logger
from wrapper_common import (
    WrapperFatalException,
    JSONFormatterOverSocket,
    AbstractWrapper,
    create_logger,
    get_parser_with_common_args,
)


class Wrapper(AbstractWrapper):
    def __init__(self,
                 logger,
                 loop,
                 handler,
                 event_socket_path,
                 control_socket_path,
                 platform_kind,
                 namespace=None,
                 worker_id=None,
                 trigger_kind=None,
                 trigger_name=None,
                 decode_event_strings=True):
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
        self._event_socket_path = event_socket_path
        self._event_sock = None

        # connect to processor (event socket)
        self._event_sock = self._connect_to_processor(self._event_socket_path)

        # make a writeable file from processor (event socket)
        self._event_sock_wfile = self._event_sock.makefile('w')

        # set socket to nonblocking to allow the asyncio event loop to run while we're waiting on a socket, and so
        # that we are able to cancel the wait if needed
        self._event_sock.setblocking(False)

        # replace the default output with the process socket
        self._logger.set_handler('default', self._event_sock_wfile, JSONFormatterOverSocket())

    async def serve_requests(self, num_requests=None):
        """Read event from socket, send out reply"""

        while True:
            try:
                # resolve event message length
                self._event_message_length_task = asyncio.create_task(
                    self._resolve_event_message_length(self._event_sock)
                )
                event_message_length = await self._event_message_length_task

                self._event_message_length_task = None

                # resolve event message
                event = await self._resolve_event(self._event_sock, event_message_length)

                # do not handle an event if a worker is drained
                if not self._discard_events:
                    try:
                        # handle event
                        await self._handle_event(event)
                    except BaseException as exc:
                        await self._on_handle_event_error(exc, self._event_sock)
                else:
                    self._logger.debug_with('Event has been discarded', event=event)

                # allow event to be garbage collected by deleting the reference
                del event

            except WrapperFatalException as exc:
                await self._on_serving_error(exc, self._event_sock)

                # explode, unrecoverable exception
                self._shutdown(error_code=1)

            except UnicodeDecodeError as exc:

                # reset unpacker to avoid consecutive errors
                # this may happen when msgpack fails to decode a non-utf8 events
                self._unpacker = self._resolve_unpacker()
                await self._on_serving_error(exc, self._event_sock)

            except asyncio.CancelledError:
                self._logger.debug('Waiting for event message was interrupted by a signal')

            except Exception as exc:
                await self._on_serving_error(exc, self._event_sock)

            finally:
                if self._is_drain_needed:
                    result = self._call_drain_handler()
                    if asyncio.iscoroutine(result):
                        await result

                if self._is_termination_needed:
                    result = self._call_termination_handler()
                    if asyncio.iscoroutine(result):
                        await result

            # for testing, we can ask wrapper to only read a set number of requests
            if num_requests is not None:
                num_requests -= 1
                if num_requests <= 0:
                    break

    async def initialize(self):

        # call init_context
        await self._initialize_context()

        # register to the SIGUSR1 and SIGUSR2 signals, used to signal termination/draining respectively
        self._register_to_signal()

        # indicate that we're ready
        await self._write_packet_to_processor(self._event_sock, 's')
        await self._send_data_on_control_socket({
            'kind': 'wrapperInitialized',
            'attributes': {'ready': 'true'}
        })

    async def _write_response_error(self, body, sock=None):
        sock = self._event_sock if not sock else sock
        await super()._write_response_error(body=body, sock=sock)

    async def _handle_event(self, event, sock=None):
        sock = self._event_sock if not sock else sock
        await super()._handle_event(event, sock)

    def _shutdown(self, error_code=0):
        try:
            self._event_sock.close()
        finally:
            super()._shutdown()


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
        wrapper_instance = Wrapper(root_logger,
                                   loop,
                                   args.handler,
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

    # 3.6-compatible alternative to asyncio.run()
    try:
        loop.run_until_complete(wrapper_instance.initialize())
        loop.run_until_complete(wrapper_instance.serve_requests())
    finally:

        # finalize all scheduled asynchronous generators reliably
        loop.run_until_complete(loop.shutdown_asyncgens())
        loop.close()


if __name__ == '__main__':
    # run the wrapper
    run_wrapper()
