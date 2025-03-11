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
import os

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
        self._platform_kind = 'test'
        self._default_test_handler = 'test_server_wrapper:random_sleep'
        self._host = "0.0.0.0"
        self._port = 1337

        self._control_message_socket_path = os.path.join(self._temp_path, 'nuclio.control')

        self._unix_stream_server, self._unix_stream_server_thread = \
            self._create_unix_stream_server(self._control_message_socket_path)

        self._wrapper = wrapper.AsyncWrapper(
            logger=self._logger,
            loop=self._loop,
            handler=self._default_test_handler,
            serving_address=f"{self._host}:{self._port}",
            control_socket_path=self._control_message_socket_path,
            platform_kind=self._platform_kind,
            decode_event_strings=self._decode_event_strings)
        self._loop.run_until_complete(self._wrapper.initialize())
        self._wrapper_run_task = self._loop.create_task(self._wrapper.start())

    def tearDown(self):

        self._wrapper._control_sock.close()

        for unix_stream_server, unix_stream_server_thread in [
            (self._unix_stream_server, self._unix_stream_server_thread),
        ]:
            unix_stream_server.server_close()
            unix_stream_server.shutdown()
            unix_stream_server_thread.join()

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
        data = self._loop.run_until_complete(self._send_events(events, single_connection=single_connection))

        # validate data in event socket
        # we expect 3/4 messages for each event
        # 's' (1 per socket) - signals that connection established
        # 'l' (1 per event) - log
        # 'm' (1 per event) - processing duration metric
        # 'r' (1 per event) - processing response
        responses_number = logs_number = starts_number = metrics_number = 0
        for messages in data:
            for message in messages.split("\n"):
                if message[0] == "s":
                    starts_number += 1
                elif message[0] == "l":
                    logs_number += 1
                elif message[0] == "m":
                    metrics_number += 1
                elif message[0] == "r":
                    responses_number += 1

        self.assertEqual(num_of_events, responses_number)
        self.assertEqual(num_of_events, metrics_number)
        self.assertEqual(num_of_events, logs_number)

        self.assertEqual(num_of_events, len(recorded_events), 'wrong number of events')
        if single_connection:
            self.assertEqual(1, starts_number)
            # we expect the event to be ordered since though the function is "asynchronous", it is blocked
            # by the processor until it gets response.
            for recorded_event_index, recorded_event in enumerate(
                    sorted(recorded_events, key=operator.attrgetter('id'))):
                self.assertEqual(recorded_event_index, recorded_event.id)
                self.assertEqual('e{}'.format(recorded_event_index), self._ensure_str(recorded_event.body))
        else:
            self.assertEqual(num_of_events, starts_number)
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
        # check that general logs are sent to the control message socket
        self._wait_until_received_messages(
            minimum_messages_length=3,
            messages=self._unix_stream_server._messages,
        )

    async def _send_events(self, events, single_connection=True):
        data = []
        if single_connection:
            client_socket = self._create_client_socket()
            for event in events:
                data.append(await self._send_event(event, client_socket))
            client_socket.close()
        else:
            tasks = []
            for event in events:
                tasks.append(asyncio.create_task(self._send_event(event)))
            data = await asyncio.gather(*tasks)
        return data

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

        data = await self.wait_for_response(client_socket)

        if close_socket_needed:
            client_socket.close()

        return data

    async def wait_for_response(self, client_socket, delimiter=b'\n', buffer_size=128):
        """Read data from a socket until a full message starting with 'r' and ending with '\n' is encountered."""
        data = bytearray()
        messages = []

        while True:
            chunk = await self._loop.sock_recv(client_socket, buffer_size)
            if not chunk:
                break

            data.extend(chunk)

            # Search for the delimiter in the received data
            split_data = data.split(delimiter)
            for message in split_data[:-1]:
                messages.append(message)
                if message.startswith(b'r'):
                    return b'\n'.join(messages).decode('utf-8')

            # Keep the remaining part if the last chunk does not end with the delimiter
            data = split_data[-1]

        # Add the last part if it is not empty
        if data:
            messages.append(data)

        return b'\n'.join(messages).decode('utf-8')

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
