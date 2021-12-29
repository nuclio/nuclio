# Copyright 2017 The Nuclio Authors.
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

import argparse
import asyncio
import json
import logging
import re
import socket
import sys
import time
import traceback

import msgpack
import nuclio_sdk
import nuclio_sdk.helpers
import nuclio_sdk.json_encoder
import nuclio_sdk.logger


class WrapperFatalException(Exception):
    """
    Wrapper fatal is an exception the wrapper can not (perhaps should not) recover from
    and will lead to wrapper termination
    """
    pass


# Appends `l` character to follow the processor conventions
# more information @ pkg/processor/runtime/rpc/abstract.go / wrapperOutputHandler
class JSONFormatterOverSocket(nuclio_sdk.logger.JSONFormatter):
    def format(self, record):
        return 'l' + super(JSONFormatterOverSocket, self).format(record)


class Wrapper(object):
    def __init__(self,
                 logger,
                 loop,
                 handler,
                 socket_path,
                 platform_kind,
                 namespace=None,
                 worker_id=None,
                 trigger_kind=None,
                 trigger_name=None,
                 decode_event_strings=True):
        self._logger = logger
        self._socket_path = socket_path
        self._json_encoder = nuclio_sdk.json_encoder.Encoder()
        self._entrypoint = None
        self._processor_sock = None
        self._platform = nuclio_sdk.Platform(platform_kind, namespace=namespace)
        self._decode_event_strings = decode_event_strings

        # 1gb
        self._max_buffer_size = 1024 * 1024 * 1024

        # holds the function that will be called
        self._entrypoint = self._load_entrypoint_from_handler(handler)

        self._is_entrypoint_coroutine = asyncio.iscoroutinefunction(self._entrypoint)

        # connect to processor
        self._processor_sock = self._connect_to_processor()

        # make a writeable file from processor
        self._processor_sock_wfile = self._processor_sock.makefile('w')

        # create msgpack unpacker
        self._unpacker = self._resolve_unpacker()

        # set event loop
        self._loop = loop

        # event deserializer kind (e.g.: msgpack_raw / json)
        self._event_deserializer_kind = self._resolve_event_deserializer_kind()

        # get handler module
        self._entrypoint_module = sys.modules[self._entrypoint.__module__]

        # create a context with logger and platform
        self._context = nuclio_sdk.Context(self._logger,
                                           self._platform,
                                           worker_id,
                                           nuclio_sdk.TriggerInfo(trigger_kind, trigger_name))

        # replace the default output with the process socket
        self._logger.set_handler('default', self._processor_sock_wfile, JSONFormatterOverSocket())

    async def serve_requests(self, num_requests=None):
        """Read event from socket, send out reply"""

        while True:

            try:

                # resolve event message length
                event_message_length = await self._resolve_event_message_length()

                # resolve event message
                event = await self._resolve_event(event_message_length)

                try:

                    # handle event
                    await self._handle_event(event)
                except BaseException as exc:
                    self._on_handle_event_error(exc)

            except WrapperFatalException as exc:
                self._on_serving_error(exc)

                # explode, unrecoverable exception
                self._shutdown(error_code=1)

            except UnicodeDecodeError as exc:

                # reset unpacker to avoid consecutive errors
                # this may happen when msgpack fails to decode a non-utf8 events
                self._unpacker = self._resolve_unpacker()
                self._on_serving_error(exc)

            except Exception as exc:
                self._on_serving_error(exc)

            # for testing, we can ask wrapper to only read a set number of requests
            if num_requests is not None:
                num_requests -= 1
                if num_requests <= 0:
                    break

    async def initialize(self):

        # call init_context
        await self._initialize_context()

        # indicate that we're ready
        self._write_packet_to_processor('s')

    async def _initialize_context(self):

        # call init context
        if hasattr(self._entrypoint_module, 'init_context'):
            try:
                init_context = getattr(self._entrypoint_module, 'init_context')
                init_context_result = getattr(self._entrypoint_module, 'init_context')(self._context)
                if asyncio.iscoroutinefunction(init_context):
                    await init_context_result

            except:
                self._logger.error('Exception raised while running init_context')
                raise

    def _resolve_unpacker(self):
        """
        Since this wrapper is behind the nuclio processor, in which pre-handle the traffic & request
        it is not mandatory to provide security over max buffer size.
        the request limit should be handled on the processor level.

        unpacker raw determines whether an incoming message would be decoded to utf8
        """
        return msgpack.Unpacker(raw=not self._decode_event_strings, max_buffer_size=self._max_buffer_size)

    def _resolve_event_deserializer_kind(self):
        """
        Event deserializer kind to use when deserializing incoming event messages
        """
        if self._decode_event_strings:
            return nuclio_sdk.event.EventDeserializerKinds.msgpack
        return nuclio_sdk.event.EventDeserializerKinds.msgpack_raw

    def _load_entrypoint_from_handler(self, handler):
        """
        Load handler function from handler.
        handler is in the format 'module.sub:handler_name'
        """
        match = re.match(r'^([\w|-]+(\.[\w|-]+)*):(\w+)$', handler)
        if not match:
            raise ValueError('Malformed handler - {!r}'.format(handler))

        module_name, entrypoint = match.group(1), match.group(3)

        module = __import__(module_name)
        for sub in module_name.split('.')[1:]:
            module = getattr(module, sub)

        try:
            entrypoint_address = getattr(module, entrypoint)
        except Exception:
            self._logger.error_with('Handler not found', handler=handler)
            raise

        return entrypoint_address

    def _connect_to_processor(self, timeout=60):
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)

        for _ in range(timeout):
            try:
                sock.connect(self._socket_path)
                return sock

            except:

                # logger isn't available yet
                print('Failed to connect to ' + self._socket_path)

                time.sleep(1)

        raise RuntimeError('Failed to connect to {0} in given timeframe'.format(self._socket_path))

    def _write_packet_to_processor(self, body):

        # TODO: make async
        self._processor_sock_wfile.write(body + '\n')
        self._processor_sock_wfile.flush()

    async def _resolve_event_message_length(self):
        """
        Determines the message body size
        """
        if self._is_entrypoint_coroutine:
            int_buf = await self._loop.sock_recv(self._processor_sock, 4)
        else:
            int_buf = self._processor_sock.recv(4)

        # not reading 4 bytes meaning client has disconnected while sending the packet. bail
        if len(int_buf) != 4:
            raise WrapperFatalException('Client disconnected')

        # big-endian, compute event bytes length to read
        bytes_to_read = int(int_buf[3])
        bytes_to_read += int_buf[2] << 8
        bytes_to_read += int_buf[1] << 16
        bytes_to_read += int_buf[0] << 24
        if bytes_to_read <= 0:
            raise WrapperFatalException('Illegal message size: {0}'.format(bytes_to_read))

        return bytes_to_read

    async def _resolve_event(self, expected_event_bytes_length):
        """
        Reading the expected event length from socket and instantiate an event message
        """

        cumulative_bytes_read = 0
        while cumulative_bytes_read < expected_event_bytes_length:
            bytes_to_read_now = expected_event_bytes_length - cumulative_bytes_read
            if self._is_entrypoint_coroutine:
                bytes_read = await self._loop.sock_recv(self._processor_sock, bytes_to_read_now)
            else:
                bytes_read = self._processor_sock.recv(bytes_to_read_now)

            if not bytes_read:
                raise WrapperFatalException('Client disconnected')

            self._unpacker.feed(bytes_read)
            cumulative_bytes_read += len(bytes_read)

        # resolve msgpack event message
        event_message = next(self._unpacker)

        # instantiate event message
        return nuclio_sdk.Event.deserialize(event_message, kind=self._event_deserializer_kind)

    def _on_serving_error(self, exc):
        self._log_and_response_error(exc, 'Exception caught while serving')

    def _on_handle_event_error(self, exc):
        self._log_and_response_error(exc, 'Exception caught in handler')

    def _log_and_response_error(self, exc, error_message):
        encoded_error_response = '{0} - "{1}": {2}'.format(error_message,
                                                           exc,
                                                           traceback.format_exc())
        self._logger.error_with(error_message, exc=str(exc), traceback=traceback.format_exc())
        self._write_response_error(encoded_error_response or error_message)

    def _write_response_error(self, body):
        try:
            encoded_response = self._json_encoder.encode({
                'body': body,
                'body_encoding': 'text',
                'content_type': 'text/plain',
                'status_code': 500,
            })

            # try write the formatted exception back to processor
            self._write_packet_to_processor('r' + encoded_response)
        except Exception as exc:
            print('Failed to write message to processor after serving error detected, is socket open?\n'
                  'Exception: {0}'.format(str(exc)))

    async def _handle_event(self, event):

        # take call time
        start_time = time.time()

        # call the entrypoint
        entrypoint_output = self._entrypoint(self._context, event)
        if self._is_entrypoint_coroutine:
            entrypoint_output = await entrypoint_output

        # measure duration, set to minimum float in case execution was too fast
        duration = time.time() - start_time or sys.float_info.min

        self._write_packet_to_processor('m' + json.dumps({'duration': duration}))

        response = nuclio_sdk.Response.from_entrypoint_output(self._json_encoder.encode,
                                                              entrypoint_output)

        # try to json encode the response
        encoded_response = self._json_encoder.encode(response)

        # write response to the socket
        self._write_packet_to_processor('r' + encoded_response)

    def _shutdown(self, error_code=0):
        print('Shutting down')
        self._processor_sock.close()
        sys.exit(error_code)

#
# init
#


def create_logger(level):
    """Create a logger that emits JSON to stdout"""

    return nuclio_sdk.Logger(level)


def parse_args():
    parser = argparse.ArgumentParser(description=__doc__)

    parser.add_argument('--handler',
                        help='handler (module.sub:handler)',
                        required=True)

    parser.add_argument('--socket-path',
                        help='path to unix socket to listen on',
                        required=True)

    parser.add_argument('--log-level',
                        help='level of logging',
                        default=logging.DEBUG)

    parser.add_argument('--platform-kind',
                        choices=['local', 'kube'],
                        default='local')

    parser.add_argument('--namespace')

    parser.add_argument('--trigger-kind')

    parser.add_argument('--trigger-name')

    parser.add_argument('--worker-id')

    parser.add_argument('--decode-event-strings',
                        action='store_true',
                        help='Decode event strings to utf8 (Decoding is done via msgpack, Default: False)')

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
                                   args.socket_path,
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
