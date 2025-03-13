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

import json

import os
import time
import socketserver
import threading
import socket

import unittest.mock


class BaseTestSubmitEvents(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls._decode_event_strings = False

    @staticmethod
    def _ensure_str(s, encoding='utf-8', errors='strict'):

        # Optimization: Fast return for the common case.
        if type(s) is str:
            return s
        if isinstance(s, bytes):
            return s.decode(encoding, errors)
        raise TypeError(f"not expecting type '{type(s)}'")

    @staticmethod
    def _event_to_dict(event):
        return json.loads(event.to_json())

    @staticmethod
    def _write_handler(temp_path):
        handler_code = '''import sys

def handler(ctx, event):
    """Return reversed body as string"""
    body = event.body
    if isinstance(event.body, bytes):
        body = event.body.decode('utf-8')
    ctx.logger.warn('the end is nigh')
    return body[::-1]
'''

        handler_path = os.path.join(temp_path, 'reverser.py')

        with open(handler_path, 'w') as out:
            out.write(handler_code)

        return handler_path

    @staticmethod
    def _create_unix_stream_server(socket_path):
        unix_stream_server = _SingleConnectionUnixStreamServer(socket_path, _Connection)

        # create a thread and listen forever on server
        unix_stream_server_thread = threading.Thread(target=unix_stream_server.serve_forever)
        unix_stream_server_thread.daemon = True
        unix_stream_server_thread.start()
        return unix_stream_server, unix_stream_server_thread

    def _wait_until_received_messages(self, minimum_messages_length, messages, timeout=10, interval=1):
        while timeout > 0:
            time.sleep(interval)
            current_messages_length = len(messages)
            if current_messages_length >= minimum_messages_length:
                return
            self._logger.debug_with('Waiting for messages to arrive',
                                    current_messages_length=current_messages_length,
                                    minimum_messages_length=minimum_messages_length)
            timeout -= interval
        raise RuntimeError('Failed waiting for messages')


class _SingleConnectionUnixStreamServer(socketserver.UnixStreamServer):

    def __init__(self, server_address, RequestHandlerClass, bind_and_activate=True):
        socketserver.UnixStreamServer.__init__(self, server_address, RequestHandlerClass, bind_and_activate)

        self._connection_socket = None  # type: socket.socket
        self._messages = []


class _Connection(socketserver.BaseRequestHandler):

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
                if line[0] != "{":
                    # event socket encoding
                    message = {
                        'type': line[0],
                        'body': json.loads(line[1:]) if line[0] != 's' else ''
                    }
                else:
                    # control message socket encoding
                    message = json.loads(line)

                self.server._messages.append(message)

            except:
                pass
