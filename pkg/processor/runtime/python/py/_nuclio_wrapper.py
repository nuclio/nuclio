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
import json
import logging
import re
import socket
import sys
import time
import traceback

import msgpack
import nuclio_sdk
import nuclio_sdk.json_encoder
import nuclio_sdk.logger


class Wrapper(object):

    _max_message_size = 4 * 1024 * 1024

    def __init__(self,
                 logger,
                 handler,
                 socket_path,
                 platform_kind,
                 namespace=None,
                 worker_id=None,
                 trigger_kind=None,
                 trigger_name=None):
        self._logger = logger
        self._socket_path = socket_path
        self._json_encoder = nuclio_sdk.json_encoder.Encoder()
        self._entrypoint = None
        self._processor_sock = None
        self._platform = nuclio_sdk.Platform(platform_kind, namespace=namespace)

        # holds the function that will be called
        self._entrypoint = self._load_entrypoint_from_handler(handler)

        # connect to processor
        self._processor_sock = self._connect_to_processor()

        # make a writeable file from processor
        self._processor_sock_wfile = self._processor_sock.makefile('w')
        self._unpacker = msgpack.Unpacker(raw=False, max_buffer_size=10 * 1024 * 1024)

        # get handler module
        entrypoint_module = sys.modules[self._entrypoint.__module__]

        # create a context with logger and platform
        self._context = nuclio_sdk.Context(self._logger,
                                           self._platform,
                                           worker_id,
                                           nuclio_sdk.TriggerInfo(trigger_kind, trigger_name))

        # call init context
        if hasattr(entrypoint_module, 'init_context'):
            try:
                getattr(entrypoint_module, 'init_context')(self._context)
            except:
                self._logger.error('Exception raised while running init_context')
                raise

        # replace the default output with the process socket
        self._logger.set_handler('default', self._processor_sock_wfile, nuclio_sdk.logger.JSONFormatter())

        # indicate that we're ready
        self._write_packet_to_processor('s')

    def serve_requests(self, num_requests=None):
        """Read event from socket, send out reply"""

        int_buf = bytearray(4)
        buf = memoryview(bytearray(self._max_message_size))

        while True:

            formatted_exception = None
            encoded_response = '{}'

            try:

                should_be_four = self._processor_sock.recv_into(int_buf, 4)
                # client disconnect
                if should_be_four < 4:
                    # If socket is done, we can't log
                    print('Client disconnect')
                    return

                bytes_to_read = int(int_buf[3])
                bytes_to_read += int_buf[2] << 8
                bytes_to_read += int_buf[1] << 16
                bytes_to_read += int_buf[0] << 24
                if bytes_to_read > self._max_message_size or bytes_to_read <= 0:
                    # If socket is done, we can't log
                    print('Illegal message size: ' + str(bytes_to_read))
                    return

                cumulative_bytes_read = 0
                while cumulative_bytes_read < bytes_to_read:
                    view = buf[cumulative_bytes_read:bytes_to_read]
                    bytes_to_read_now = bytes_to_read - cumulative_bytes_read
                    bytes_read = self._processor_sock.recv_into(view, bytes_to_read_now)

                    # client disconnect
                    if not bytes_read:
                        # If socket is done, we can't log
                        print('Client disconnect')
                        return

                    cumulative_bytes_read += bytes_read

                self._unpacker.feed(buf[:cumulative_bytes_read])

                msg = next(self._unpacker)

                # decode the event
                event = nuclio_sdk.Event.from_msgpack(msg)

                try:

                    # take call time
                    start_time = time.time()

                    # call the entrypoint
                    entrypoint_output = self._entrypoint(self._context, event)

                    # measure duration
                    duration = time.time() - start_time

                    self._write_packet_to_processor('m' + json.dumps({'duration': duration}))

                    response = nuclio_sdk.Response.from_entrypoint_output(self._json_encoder.encode,
                                                                          entrypoint_output)

                    # try to json encode the response
                    encoded_response = self._json_encoder.encode(response)

                except BaseException as exc:
                    formatted_exception = 'Exception caught in handler "{0}": {1}'.format(exc, traceback.format_exc())

            except Exception as exc:
                formatted_exception = 'Exception caught while serving "{0}": {1}'.format(exc, traceback.format_exc())

            # if we have a formatted exception, return it as 500
            if formatted_exception is not None:
                self._logger.warn(formatted_exception)

                encoded_response = self._json_encoder.encode({
                    'body': formatted_exception,
                    'body_encoding': 'text',
                    'content_type': 'text/plain',
                    'status_code': 500,
                })

            # write to the socket
            self._write_packet_to_processor('r' + encoded_response)

            # for testing, we can ask wrapper to only read a set number of requests
            if num_requests is not None and num_requests != 0:
                num_requests -= 1

            if num_requests == 0:
                break

    def _load_entrypoint_from_handler(self, handler):
        """
        Load handler function from handler.
        handler is in the format 'module.sub:handler_name'
        """
        match = re.match('^([\w|-]+(\.[\w|-]+)*):(\w+)$', handler)
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
        self._processor_sock_wfile.write(body + '\n')
        self._processor_sock_wfile.flush()

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

    try:

        # create a new wrapper
        wrapper_instance = Wrapper(root_logger,
                                   args.handler,
                                   args.socket_path,
                                   args.platform_kind,
                                   args.namespace,
                                   args.worker_id,
                                   args.trigger_kind,
                                   args.trigger_name)

    except BaseException as exc:
        root_logger.error_with('Caught unhandled exception while initializing',
                               err=str(exc),
                               traceback=traceback.format_exc())

        raise SystemExit(1)

    # register the function @ the wrapper
    wrapper_instance.serve_requests()


if __name__ == '__main__':

    # run the wrapper
    run_wrapper()
