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

import os
import unittest
import sys
import threading
import tempfile
import json
import time
import logging

import nuclio_sdk.wrapper


# python2/3 differences
if sys.version_info[:2] >= (3, 0):
    from socketserver import UnixStreamServer, BaseRequestHandler
else:
    from SocketServer import UnixStreamServer, BaseRequestHandler, StreamRequestHandler


class TestSubmitEvents(unittest.TestCase):

    def setUp(self):
        self._temp_path = tempfile.mkdtemp()

        # write handler to temp path
        self._handler_path = self._write_handler(self._temp_path)

        # set PYTHONPATH to include temp path
        sys.path.append(self._temp_path)

        # generate socket path
        self._socket_path = os.path.join(self._temp_path, 'nuclio.sock')

        # create transport
        self._unix_stream_server = self._create_unix_stream_server(self._socket_path)

        # create logger
        self._logger = nuclio_sdk.Logger(logging.DEBUG)
        self._logger.set_handler('default', sys.stdout, nuclio_sdk.logger.HumanReadableFormatter())

        # create a wrapper
        self._wrapper = nuclio_sdk.wrapper.Wrapper(self._logger, 'reverser:handler', self._socket_path, 'test')

    def tearDown(self):
        sys.path.remove(self._temp_path)

        self._unix_stream_server.shutdown()

    def test_event(self):
        event = nuclio_sdk.Event(body='reverse this')

        time.sleep(1)

        # write the event to the transport
        self._unix_stream_server._connection_socket.send(event.to_json() + '\n')

        # handle one request
        self._wrapper.serve_requests(num_requests=1)

        time.sleep(3)

        print self._unix_stream_server._messages

    def _create_unix_stream_server(self, socket_path):
        unix_stream_server = _SingleConnectionUnixStreamServer(socket_path, _Connection)

        # create a thread and listen forever on server
        unix_stream_server_thread = threading.Thread(target=unix_stream_server.serve_forever)
        unix_stream_server_thread.daemon = True
        unix_stream_server_thread.start()

        return unix_stream_server

    def _write_handler(self, temp_path):
        handler_code = '''import sys

is_py2 = sys.version_info[:2] < (3, 0)

def handler(ctx, event):
    """Return reversed body as string"""
    body = event.body
    if not is_py2:
        body = body.decode('utf-8')
    ctx.logger.warn('the end is nigh')
    return body[::-1]
'''

        handler_path = os.path.join(temp_path, 'reverser.py')

        with open(handler_path, 'w') as out:
            out.write(handler_code)

        return handler_path


class _SingleConnectionUnixStreamServer(UnixStreamServer):

    def __init__(self, server_address, RequestHandlerClass, bind_and_activate=True):
        UnixStreamServer.__init__(self, server_address, RequestHandlerClass, bind_and_activate)

        self._connection_socket = None
        self._messages = []


class _Connection(BaseRequestHandler):

    def handle(self):
        self.request.settimeout(1)

        # make a file from the socket so we can readln
        socket_file = self.request.makefile('r')

        # save the connection socket
        self.server._connection_socket = self.request

        # while the server isn't shut down
        while not self.server._BaseServer__shutdown_request:

            try:
                line = socket_file.readline()
                if not line:
                    continue

                message = {
                    'type': line[0],
                    'body': json.loads(line[1:])
                }

                self.server._messages.append(message)

            except:
                pass

