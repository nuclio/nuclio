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
import logging
import operator
import socket
import struct
import sys
import tempfile
import random
import unittest

import msgpack
import nuclio_sdk
import nuclio_sdk.helpers
import collections

import pytest

import _nuclio_async_wrapper as wrapper
from test_base import BaseTestSubmitEvents
from wrapper_common import WrapperFatalException


async def random_sleep(context, event):
    # random execution time
    await asyncio.sleep(random.uniform(0.1, 1))
    return 'ok'

def sync_handler():
    return "wrong handler"

class TestSubmitEvents(BaseTestSubmitEvents):

    def setUp(self):
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        self._loop = loop
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
        self._default_test_handler = 'test_server_wrapper:random_sleep'
        self._host = "0.0.0.0"
        self._port = 1337

        self._wrapper = wrapper.AsyncWrapper(
            logger=self._logger,
            loop=self._loop,
            handler=self._default_test_handler,
            serving_address=f"{self._host}:{self._port}",
            control_socket_path=None,
            platform_kind=self._platform_kind,
            decode_event_strings=self._decode_event_strings)
        self._loop.run_until_complete(self._wrapper.initialize())
        self._wrapper_run_task = self._loop.create_task(self._wrapper.start())

    def tearDown(self):
        sys.path.remove(self._temp_path)
        asyncio.run(self._wrapper._stop_processing())
        self._wrapper_run_task.cancel()
        self._loop.close()

    def test_async_handler_single_connection(self):
        self._test_async_handler(single_connection=True)

    def test_async_handler_multiple_connections(self):
        self._test_async_handler(single_connection=False)

    def _test_async_handler(self, single_connection):
        """Test function decorated with async and running an event loop"""

        recorded_events = []

        async def event_recorder(context, event):
            # random execution time
            await asyncio.sleep(random.uniform(0.1, 1))
            recorded_events.append(event)
            context.logger.debug_with('appending event', event=repr(event.id))
            return 'ok'

        num_of_events = 10
        events = (
            nuclio_sdk.Event(_id=i, body='e{}'.format(i))
            for i in range(num_of_events)
        )
        self._wrapper._entrypoint = event_recorder
        self._loop.run_until_complete(self._send_events(events, single_connection=single_connection))

        self.assertEqual(num_of_events, len(recorded_events), 'wrong number of events')
        if single_connection:
            # we expect the event to be ordered since though the function is "asynchronous", it is blocked
            # by the processor until it gets response.
            for recorded_event_index, recorded_event in enumerate(
                    sorted(recorded_events, key=operator.attrgetter('id'))):
                self.assertEqual(recorded_event_index, recorded_event.id)
                self.assertEqual('e{}'.format(recorded_event_index), self._ensure_str(recorded_event.body))
        else:
            expected_events = [
                {'id': i, 'body': f'e{i}'}
                for i in range(num_of_events)
            ]
            actual_events = [
                {'id': recorded_event.id, 'body': self._ensure_str(recorded_event.body)}
                for recorded_event in recorded_events
            ]
            self.assertEqual(
                collections.Counter(map(frozenset, expected_events)),
                collections.Counter(map(frozenset, actual_events)),
                "Recorded events do not match the expected events"
            )

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

        await self.read_until_delimiter(client_socket)
        await self.read_until_delimiter(client_socket)

        if close_socket_needed:
            client_socket.close()

    async def read_until_delimiter(self, client_socket, delimiter=b'\n', buffer_size=128):
        """Read data from a socket until the specified delimiter is encountered."""
        data = bytearray()
        while True:
            chunk = await self._loop.sock_recv(client_socket, buffer_size)
            if not chunk:
                raise ConnectionError("Socket closed before receiving a complete message.")

            data.extend(chunk)

            # Search for the delimiter in the received data
            delimiter_index = data.find(delimiter)
            if delimiter_index != -1:
                # Extract the message up to the delimiter
                message = data[:delimiter_index]
                # Save the remaining data for future reads
                self._remaining_data = data[delimiter_index + len(delimiter):]
                return message.decode('utf-8')

    def _create_client_socket(self):
        client_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        client_socket.connect((self._host, self._port))
        client_socket.setblocking(False)
        return client_socket


class TestWrapperValidation(unittest.TestCase):

    @classmethod
    def setUpClass(cls) -> None:
        cls._logger = nuclio_sdk.Logger(logging.DEBUG)
        cls._decode_event_strings = False
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        cls._loop = loop
        cls._platform_kind = 'test'

    def test_invalid_handler(self):
        with pytest.raises(WrapperFatalException):
            wrapper.AsyncWrapper(
                logger=self._logger,
                loop=self._loop,
                handler="test_server_wrapper:sync_handler",
                serving_address="0.0.0.0:1337",
                control_socket_path=None,
                platform_kind=self._platform_kind,
                decode_event_strings=self._decode_event_strings)
