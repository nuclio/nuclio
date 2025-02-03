# Copyright 2025 The Nuclio Authors.
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
import signal
import re
import socket
import time
import msgpack
import nuclio_sdk
import nuclio_sdk.helpers
import nuclio_sdk.json_encoder
import sys
import asyncio
import functools
import argparse
import logging
import traceback
import json


class Constants:
    # in msgpack protoctol, binary messages' length is 4 bytes long
    msgpack_message_length_bytes = 4

    termination_signal = signal.SIGUSR1
    drain_signal = signal.SIGUSR2
    continue_signal = signal.SIGCONT


class WrapperFatalException(Exception):
    """
    Wrapper fatal is an exception the wrapper can not (perhaps should not) recover from
    and will lead to wrapper termination
    """
    pass


# Appends `l` character to follow the processor conventions for "log"
# more information @ pkg/processor/runtime/rpc/abstract.go / eventWrapperOutputHandler
class JSONFormatterOverSocket(nuclio_sdk.logger.JSONFormatter):
    def format(self, record):
        return 'l' + super(JSONFormatterOverSocket, self).format(record)


class AbstractWrapper(object):
    def __init__(self,
                 logger,
                 loop,
                 handler,
                 control_socket_path,
                 platform_kind,
                 namespace=None,
                 worker_id=None,
                 trigger_kind=None,
                 trigger_name=None,
                 decode_event_strings=True):
        self._logger = logger
        self._control_socket_path = control_socket_path
        self._json_encoder = nuclio_sdk.json_encoder.Encoder()
        self._entrypoint = None
        self._control_sock = None
        self._platform = nuclio_sdk.Platform(platform_kind,
                                             namespace=namespace,
                                             on_control_callback=self._send_data_on_control_socket)
        self._decode_event_strings = decode_event_strings

        # 1gb
        self._max_buffer_size = 1024 * 1024 * 1024

        # holds the function that will be called
        self._entrypoint = self._load_entrypoint_from_handler(handler)
        # connect to processor
        # emtpy only in tests
        if self._control_socket_path:
            self._control_sock = self._connect_to_processor(self._control_socket_path)

            self._control_sock_wfile = self._control_sock.makefile('w')

            # set socket to nonblocking to allow the asyncio event loop to run while we're waiting on a socket, and so
            # that we are able to cancel the wait if needed
            self._control_sock.setblocking(False)

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

        # initialize flags
        self._is_drain_needed = False
        self._is_termination_needed = False
        self._discard_events = False

        self._event_message_length_task = None

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

    def _connect_to_processor(self, socket_path, timeout=60):
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        for _ in range(timeout):
            try:
                # TODO: remove this log when support multiple sockets as it can be too spammy
                self._logger.debug(f"Connecting to socket {socket_path}")
                sock.connect(socket_path)
                # TODO: remove this log when support multiple sockets as it can be too spammy
                self._logger.debug(f"Successfully connected to socket {socket_path}")
                return sock

            except:

                # logger isn't available yet
                self._logger.error('Failed to connect to ' + socket_path)

                time.sleep(1)

        raise RuntimeError('Failed to connect to {0} in given timeframe'.format(socket_path))

    async def _handle_event(self, event, sock):
        # take call time
        start_time = time.time()

        # call the entrypoint
        entrypoint_output = self._entrypoint(self._context, event)
        if asyncio.iscoroutine(entrypoint_output):
            entrypoint_output = await entrypoint_output

        # measure duration, set to minimum float in case execution was too fast
        duration = time.time() - start_time or sys.float_info.min

        await self._write_packet_to_processor(sock, 'm' + json.dumps({'duration': duration}))

        # try to json encode the response
        encoded_response = self._encode_entrypoint_output(entrypoint_output)

        # write response to the socket
        await self._write_packet_to_processor(sock, 'r' + encoded_response)

    def _encode_entrypoint_output(self, entrypoint_output):

        # processing entrypoint output if response is batched
        if isinstance(entrypoint_output, list):
            response = [nuclio_sdk.Response.from_entrypoint_output(self._json_encoder.encode,
                                                                   _output) for _output in entrypoint_output]
        else:
            response = nuclio_sdk.Response.from_entrypoint_output(self._json_encoder.encode, entrypoint_output)

        # try to json encode the response
        return self._json_encoder.encode(response)

    async def _send_data_on_control_socket(self, data):
        if not self._control_sock:
            return
        self._logger.debug_with('Sending data on control socket', data_length=len(data))

        # send message to processor
        encoded_offset_data = self._json_encoder.encode(data)
        await self._write_packet_to_processor(self._control_sock, encoded_offset_data)

        # TODO: wait for response that processor received data

    async def _write_packet_to_processor(self, sock, body):
        await self._loop.sock_sendall(sock, (body + '\n').encode('utf-8'))

    async def _receive_control_messages(self):

        control_message_event_length = await self._resolve_event_message_length(self._control_sock)

        control_message_event = await self._resolve_event(self._control_sock, control_message_event_length)

        self._logger.debug_with('Received control message', control_message=control_message_event.body)

    async def _resolve_event_message_length(self, sock):
        """
        Determines the message body size
        """
        int_buf = await self._loop.sock_recv(sock, Constants.msgpack_message_length_bytes)

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

    async def _resolve_event(self, sock, expected_event_bytes_length):
        """
        Reading the expected event length from socket and instantiate an event message
        """
        cumulative_bytes_read = 0
        while cumulative_bytes_read < expected_event_bytes_length:
            bytes_to_read_now = expected_event_bytes_length - cumulative_bytes_read
            bytes_read = await self._loop.sock_recv(sock, bytes_to_read_now)

            if not bytes_read:
                raise WrapperFatalException('Client disconnected')

            self._unpacker.feed(bytes_read)
            cumulative_bytes_read += len(bytes_read)

        # resolve msgpack event message
        event_message = next(self._unpacker)

        # instantiate event message
        return nuclio_sdk.Event.deserialize(event_message, kind=self._event_deserializer_kind)

    async def _on_serving_error(self, exc, sock):
        await self._log_and_response_error(exc, 'Exception caught while serving', sock)

    async def _on_handle_event_error(self, exc, sock):
        await self._log_and_response_error(exc, 'Exception caught in handler', sock)

    async def _log_and_response_error(self, exc, error_message, sock):
        encoded_error_response = '{0} - "{1}": {2}'.format(error_message,
                                                           exc,
                                                           traceback.format_exc())
        self._logger.error_with(error_message, exc=str(exc), traceback=traceback.format_exc())
        await self._write_response_error(encoded_error_response or error_message, sock)

    async def _write_response_error(self, body, sock):
        try:
            encoded_response = self._json_encoder.encode({
                'body': body,
                'body_encoding': 'text',
                'content_type': 'text/plain',
                'status_code': 500,
            })

            # try write the formatted exception back to processor
            await self._write_packet_to_processor(sock, 'r' + encoded_response)
        except Exception as exc:
            print('Failed to write message to processor after serving error detected, is socket open?\n'
                  'Exception: {0}'.format(str(exc)))

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

    def _register_to_signal(self):
        on_termination_signal = functools.partial(self._on_termination_signal, Constants.termination_signal.name)
        on_drain_signal = functools.partial(self._on_drain_signal, Constants.drain_signal.name)
        on_continue_signal = functools.partial(self._on_continue_signal, Constants.continue_signal.name)

        asyncio.get_running_loop().add_signal_handler(Constants.termination_signal, on_termination_signal)
        asyncio.get_running_loop().add_signal_handler(Constants.drain_signal, on_drain_signal)
        asyncio.get_running_loop().add_signal_handler(Constants.continue_signal, on_continue_signal)

    def _on_drain_signal(self, signal_name):
        # do not perform draining if discarding events
        if self._discard_events:
            self._logger.debug('Draining signal is received, but it will be ignored as the worker is already drained')
            return

        self._logger.debug_with('Received signal', signal=signal_name)
        self._is_drain_needed = True

        # set the flag to True to stop processing events which are received after draining
        self._discard_events = True

        # if serving loop is waiting for an event, unblock this operation to allow the drain callback to be called
        if self._event_message_length_task:
            self._event_message_length_task.cancel()

    def _on_termination_signal(self, signal_name):
        self._logger.debug_with('Received signal', signal=signal_name)
        self._is_termination_needed = True

        # set the flag to True to stop processing events which are received after termination signal
        self._discard_events = True

        # if serving loop is waiting for an event, unblock this operation to allow the termination callback to be called
        if self._event_message_length_task:
            self._event_message_length_task.cancel()

    def _on_continue_signal(self, signal_name):
        self._logger.debug_with('Received signal', signal=signal_name)

        # set this flag to False, so continue normal event processing flow
        self._discard_events = False

    def _call_drain_handler(self):
        self._logger.debug('Calling platform drain handler')

        # set the flag to False so the drain handler will not be called more than once
        self._is_drain_needed = False
        return self._platform._on_signal(callback_type="drain")

    def _call_termination_handler(self):
        self._logger.debug('Calling platform termination handler')

        # set the flag to False so the termination handler will not be called more than once
        self._is_termination_needed = False

        # call termination handler
        # TODO: send a control message to the processor after this line,
        # to indicate that the termination handler has finished, and the processor can exit early
        return self._platform._on_signal(callback_type="termination")

    def _shutdown(self, error_code=0):
        self._logger.info("Shutting down...")
        try:
            if self._control_sock:
                self._control_sock.close()
        finally:
            sys.exit(error_code)


def create_logger(level):
    """Create a logger that emits JSON to stdout"""

    return nuclio_sdk.Logger(level)


def get_parser_with_common_args():
    parser = argparse.ArgumentParser(description=__doc__)

    parser.add_argument('--handler',
                        help='handler (module.sub:handler)',
                        required=True)

    parser.add_argument('--event-socket-path',
                        help='path to unix socket to listen on',
                        required=True)

    parser.add_argument('--control-socket-path',
                        help='path to unix socket to send the processor messages on',
                        default=None)
    # required=True)

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

    return parser
