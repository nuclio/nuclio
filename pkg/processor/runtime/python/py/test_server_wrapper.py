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
import asyncio
import functools
import http.client
import json
import logging
import operator
import os
import socket
import socketserver
import struct
import sys
import tempfile
import threading
import time
import unittest.mock
import random

import msgpack
import nuclio_sdk
import nuclio_sdk.helpers

import _nuclio_wrapper_as_server as wrapper
from test_base import BaseTestSubmitEvents


class TestSubmitEvents(BaseTestSubmitEvents):

    def setUp(self):
        self._loop = asyncio.get_event_loop()
        self._loop.set_debug(True)

        self._temp_path = tempfile.mkdtemp(prefix='nuclio-test-py-server-wrapper')

        # write handler to temp path
        self._handler_path = self._write_handler(self._temp_path)

        # set PYTHONPATH to include temp path
        sys.path.append(self._temp_path)

        # create logger
        self._logger = nuclio_sdk.Logger(logging.DEBUG)
        self._logger.set_handler('test-default', sys.stdout, nuclio_sdk.logger.HumanReadableFormatter())

        self._platform_kind = 'test'
        self._default_test_handler = 'reverser:handler'
        self._host = "0.0.0.0"
        self._port = 1337

        self._wrapper = wrapper.Wrapper(
            logger=self._logger,
            loop=self._loop,
            handler=self._default_test_handler,
            serving_address=f"{self._host}:{self._port}",
            control_socket_path=None,
            platform_kind=self._platform_kind,
            decode_event_strings=self._decode_event_strings)
        self._loop.run_until_complete(self._wrapper.initialize())

    def tearDown(self):
        sys.path.remove(self._temp_path)
        self._wrapper._shutdown()

    def test_async_handler_single_connection(self):
        self._test_async_handler(single_connection=True)

    def _test_async_handler(self, single_connection):
        """Test function decorated with async and running an event loop"""

        recorded_events = []

        async def event_recorder(context, event):
            async def append_event(_event):
                context.logger.debug_with('sleeping', event=repr(_event.id))
                await asyncio.sleep(random.uniform(0.1, 1))
                context.logger.debug_with('appending event', event=repr(_event.id))
                recorded_events.append(_event)

            await asyncio.sleep(random.uniform(0.1, 1))
            # Deprecated. To be removed on nuclio > 1.18
            # using `ensure_future` to BC with python:3.6 (on >= 3.7, you will see "create_task")
            # https://docs.python.org/3/library/asyncio-task.html#asyncio.create_task
            asyncio.ensure_future(append_event(event), loop=self._loop)
            return 'ok'

        num_of_events = 10
        events = (
            nuclio_sdk.Event(_id=i, body='e{}'.format(i))
            for i in range(num_of_events)
        )
        self._wrapper._entrypoint = event_recorder
        wrapper_run_task = self._loop.create_task(self._wrapper.start())
        self._loop.run_until_complete(self._send_events(events, single_connection=single_connection))

        self.assertEqual(num_of_events, len(recorded_events), 'wrong number of events')
        if single_connection:
            # we expect the event to be ordered since though the function is "asynchronous", it is blocked
            # by the processor until it gets response.
            for recorded_event_index, recorded_event in enumerate(sorted(recorded_events, key=operator.attrgetter('id'))):
                self.assertEqual(recorded_event_index, recorded_event.id)
                self.assertEqual('e{}'.format(recorded_event_index), self._ensure_str(recorded_event.body))

        wrapper_run_task.cancel()

    async def _send_events(self, events, single_connection=True):
        if single_connection:
            client_socket = self._create_client_socket()
            for event in events:
                await self._send_event(event, client_socket)
            client_socket.close()
        else:
            tasks = []
            for event in events:
                tasks.append(asyncio.create_task(self._send_event(event)))
            await asyncio.gather(*tasks)

    async def _send_event(self, event, client_socket=None):
        close_socket_needed = False
        if not client_socket:
            client_socket = self._create_client_socket()
            close_socket_needed = True
        if not isinstance(event, dict):
            event = self._event_to_dict(event)

        # event to a msgpack body message
        body = msgpack.Packer().pack(event)

        # big endian body len
        body_len = struct.pack(">I", len(body))

        # first write body length
        await self._loop.sock_sendall(client_socket, body_len)

        # then write body content
        await self._loop.sock_sendall(client_socket, body)

        x = await self._loop.sock_recv(client_socket, 1024)
        x = await self._loop.sock_recv(client_socket, 1024)

    def _create_client_socket(self):
        client_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        client_socket.connect((self._host, self._port))
        client_socket.setblocking(False)
        return client_socket

