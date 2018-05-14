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
import time
import traceback
import sys

import nuclio_sdk
import nuclio_sdk.json_encoder
import nuclio_sdk.logger


class Wrapper(object):

    def __init__(self, logger, handler, socket_path, platform_kind, namespace=''):
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

        # replace the default output with the process socket
        self._logger.set_handler('default', self._processor_sock.makefile('w'), nuclio_sdk.logger.JSONFormatter())

        # get handler module
        entrypoint_module = sys.modules[self._entrypoint.__module__]

        # create a context with logger and platform
        self._context = nuclio_sdk.Context(self._logger, self._platform)

        # call init context
        if hasattr(entrypoint_module, 'init_context'):
            getattr(entrypoint_module, 'init_context')(self._context)

    def serve_requests(self, num_requests=None):
        """Read event from socket, send out reply"""
        buf = []

        stream = self._processor_sock.makefile('w')

        while True:

            formatted_exception = None
            encoded_response = '{}'

            try:

                # try to read a packet (delimited by \n) from the wire
                buf, packet = self._get_next_packet(self._processor_sock, buf)

                # we could've received partial data. read more in this case
                if packet is None:
                    continue

                # decode the JSON encoded event
                event = nuclio_sdk.Event.from_json(packet)

                try:

                    # take call time
                    start_time = time.time()

                    # call the entrypoint
                    entrypoint_output = self._entrypoint(self._context, event)

                    # measure duration
                    duration = time.time() - start_time

                    stream.write('m' + json.dumps({'duration': duration}) + '\n')
                    stream.flush()

                    response = nuclio_sdk.Response.from_entrypoint_output(self._json_encoder.encode,
                                                                          entrypoint_output)

                    # try to json encode the response
                    encoded_response = self._json_encoder.encode(response)

                except Exception as err:
                    formatted_exception = \
                        'Exception caught in handler "{0}": {1}'.format(
                            err, traceback.format_exc())

            except Exception as err:
                formatted_exception = \
                    'Exception caught while serving "{0}": {1}'.format(
                        err, traceback.format_exc())

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
            stream.write('r' + encoded_response + '\n')
            stream.flush()

            # for testing, we can ask wrapper to only read a set number of requests
            if num_requests is not None and num_requests != 0:
                num_requests -= 1

            if num_requests == 0:
                break

    def _load_entrypoint_from_handler(self, handler):
        """Load handler function from handler.

        handler is in the format 'module.sub:handler_name'
        """
        match = re.match('^([\w|-]+(\.[\w|-]+)*):(\w+)$', handler)
        if not match:
            raise ValueError('Malformed handler - {!r}'.format(handler))

        module_name, entrypoint = match.group(1), match.group(3)

        module = __import__(module_name)
        for sub in module_name.split('.')[1:]:
            module = getattr(module, sub)

        return getattr(module, entrypoint)

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

    def _get_next_packet(self, sock, buf):
        chunk = sock.recv(1024)

        if not chunk:
            raise socket.error('Failed to read from socket (empty chunk)')

        i = chunk.find(b'\n')
        if i == -1:
            buf.append(chunk)
            return buf, None

        packet = b''.join(buf) + chunk[:i]

        # Reset buffer
        buf = []
        buf.append(chunk[i+1:])

        if isinstance(packet, bytes):
            packet = packet.decode('utf-8')

        return buf, packet

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

    return parser.parse_args()


def run_wrapper():

    # parse arguments
    args = parse_args()

    # create a logger instance. note: there are no outputters until socket is created
    root_logger = create_logger(args.log_level)

    # add a logger output that is human readable. we'll remove it once we have a socket output. this
    # way all output goes to stdout until a socket is available and then switches exclusively to socket
    root_logger.set_handler('default', sys.stdout, nuclio_sdk.logger.HumanReadableFormatter())

    try:

        # create a new wrapper
        wrapper_instance = Wrapper(root_logger,
                                   args.handler,
                                   args.socket_path,
                                   args.platform_kind,
                                   args.namespace)

    except Exception as err:
        root_logger.warn_with('Caught unhandled exception while initializing',
                              err=err,
                              traceback=traceback.format_exc())

        raise SystemExit(1)

    # register the function @ the wrapper
    wrapper_instance.serve_requests()


if __name__ == '__main__':

    # run the wrapper
    run_wrapper()
