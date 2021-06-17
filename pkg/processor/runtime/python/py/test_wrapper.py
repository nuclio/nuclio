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

import msgpack
import nuclio_sdk
import nuclio_sdk.helpers

import _nuclio_wrapper as wrapper


class TestSubmitEvents(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        cls._decode_event_strings = False

    def setUp(self):
        self._loop = asyncio.get_event_loop()
        self._loop.set_debug(True)

        self._temp_path = tempfile.mkdtemp(prefix='nuclio-test-py-wrapper')

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
        self._logger.set_handler('test-default', sys.stdout, nuclio_sdk.logger.HumanReadableFormatter())

        self._platform_kind = 'test'
        self._default_test_handler = 'reverser:handler'

        # create a wrapper
        self._wrapper = wrapper.Wrapper(self._logger,
                                        self._loop,
                                        self._default_test_handler,
                                        self._socket_path,
                                        self._platform_kind,
                                        decode_event_strings=self._decode_event_strings)
        self._loop.run_until_complete(self._wrapper.initialize())

    def tearDown(self):
        sys.path.remove(self._temp_path)
        self._wrapper._processor_sock.close()
        self._unix_stream_server.server_close()
        self._unix_stream_server.shutdown()
        self._unix_stream_server_thread.join()

    def test_async_handler(self):
        """Test function decorated with async and running an event loop"""

        recorded_events = []

        async def event_recorder(context, event):
            async def append_event(_event):
                context.logger.debug_with('sleeping', event=repr(_event.id))
                await asyncio.sleep(0)
                context.logger.debug_with('appending event', event=repr(_event.id))
                recorded_events.append(_event)

            await asyncio.sleep(0)

            # using `ensure_future` to BC with python:3.6 (on >= 3.7, you will see "create_task")
            # https://docs.python.org/3/library/asyncio-task.html#asyncio.create_task
            asyncio.ensure_future(append_event(event), loop=self._loop)
            return 'ok'

        num_of_events = 10
        events = (
            nuclio_sdk.Event(_id=i, body='e{}'.format(i))
            for i in range(num_of_events)
        )
        self._send_events(events)
        self._wrapper._is_entrypoint_coroutine = True
        self._wrapper._entrypoint = event_recorder
        self._wrapper._processor_sock.setblocking(False)
        self._loop.run_until_complete(self._wrapper.serve_requests(num_of_events))
        self._loop.run_until_complete(self._loop.shutdown_asyncgens())
        self.assertEqual(num_of_events, len(recorded_events), 'wrong number of events')

        # we expect the event to be ordered since though the function is "asynchronous", it is blocked
        # by the processor until it gets response.
        for recorded_event_index, recorded_event in enumerate(sorted(recorded_events, key=operator.attrgetter('id'))):
            self.assertEqual(recorded_event_index, recorded_event.id)
            self.assertEqual('e{}'.format(recorded_event_index), self._ensure_str(recorded_event.body))

    def test_non_utf8_headers(self):
        """
        This test validates the expected behavior for a non-utf8 event field contents
        It sends 3 events, whereas the middle one has non-utf8 contents.
        Should allow non-utf8 when NOT decoding utf8 and throw exception when trying to decode it
        :return:
        """
        self._wait_for_socket_creation()
        self._wrapper._entrypoint = lambda context, event: self._ensure_str(event.body)

        events = [
            json.loads(nuclio_sdk.Event(_id=str(i), body='e{0}'.format(i)).to_json())
            for i in range(3)
        ]

        # middle event is malformed
        malformed_event_index = len(events) // 2
        events[malformed_event_index]['headers']['x-nuclio'] = b'\xda'

        # send events
        t = threading.Thread(target=self._send_events, args=(events,))
        t.start()

        asyncio.get_event_loop().run_until_complete(self._wrapper.serve_requests(num_requests=len(events)))
        t.join()

        # processor start
        # duration
        # function response
        # malformed log line (wrapper)
        # malformed response
        # duration
        # function response
        expected_messages = 7

        self._wait_until_received_messages(expected_messages)

        malformed_response = self._unix_stream_server._messages[-3]['body']

        if self._decode_event_strings:

            # msgpack would fail decoding a non utf8 string when deserializing the event
            self.assertEqual(http.client.INTERNAL_SERVER_ERROR, malformed_response['status_code'])
        else:
            self.assertEqual(http.client.OK, malformed_response['status_code'])
            self.assertEqual(events[malformed_event_index]['body'], malformed_response['body'])

        # ensure messages coming after malformed request are still valid
        last_function_response = self._unix_stream_server._messages[-1]['body']
        self.assertEqual(http.client.OK, last_function_response['status_code'])
        self.assertEqual(events[-1]['body'], last_function_response['body'])

    def test_bad_function_code(self):
        def raise_exception(ctx, event):
            raise RuntimeError(error_message)

        error_message = 'Im a bad entrypoint'
        self._wait_for_socket_creation()
        self._send_event(nuclio_sdk.Event(_id='1'))

        self._wrapper._entrypoint = raise_exception
        asyncio.get_event_loop().run_until_complete(self._wrapper.serve_requests(num_requests=1))

        # processor start, function log line, response body
        self._wait_until_received_messages(3)

        # extract the response
        response = next(message['body']
                        for message in self._unix_stream_server._messages
                        if message['type'] == 'r')
        response_body = response['body']
        self.assertIn(error_message, response_body)

    def test_event_illegal_message_size(self):
        def _send_illegal_message_size():
            self._unix_stream_server._connection_socket.sendall(struct.pack(">I", 0))

        self._wait_for_socket_creation()
        t = threading.Thread(target=_send_illegal_message_size)
        t.start()

        self._wrapper._entrypoint = unittest.mock.MagicMock()
        self._wrapper._entrypoint.assert_not_called()
        with self.assertRaises(SystemExit):
            asyncio.get_event_loop().run_until_complete(self._wrapper.serve_requests(num_requests=1))
        t.join()

    def test_single_event(self):
        reverse_text = 'reverse this'

        # send the event
        self._wait_for_socket_creation()
        t = threading.Thread(target=self._send_event, args=(nuclio_sdk.Event(_id=1, body=reverse_text),))
        t.start()

        asyncio.get_event_loop().run_until_complete(self._wrapper.serve_requests(num_requests=1))
        t.join()

        # processor start, function log line, response body, duration messages
        self._wait_until_received_messages(4)

        # extract the response
        response = next(message['body']
                        for message in self._unix_stream_server._messages
                        if message['type'] == 'r')
        response_body = response['body'][::-1]
        self.assertEqual(reverse_text, response_body)

    def test_blast_events(self):
        """Test when many >> 10 events are being sent in parallel"""

        def record_event(recorded_events, ctx, event):
            recorded_events.add(event.id)

        recorded_event_ids = set()
        expected_events_length = 10000

        events = (
            nuclio_sdk.Event(_id=i, body='e{}'.format(i))
            for i in range(expected_events_length)
        )

        t = threading.Thread(target=self._send_events, args=(events,))
        t.start()

        self._wrapper._entrypoint = functools.partial(record_event, recorded_event_ids)
        asyncio.get_event_loop().run_until_complete(self._wrapper.serve_requests(num_requests=expected_events_length))
        t.join()

        # record incoming events
        self.assertEqual(expected_events_length, len(recorded_event_ids), 'Wrong number of events')

    def test_multi_event(self):
        """Test when two events fit inside on TCP packet"""
        recorded_events = []

        def event_recorder(ctx, event):
            recorded_events.append(event)
            return 'OK'

        num_of_events = 10
        events = (
            nuclio_sdk.Event(_id=i, body='e{}'.format(i))
            for i in range(num_of_events)
        )
        self._send_events(events)
        self._wrapper._entrypoint = event_recorder
        asyncio.get_event_loop().run_until_complete(self._wrapper.serve_requests(num_of_events))
        self.assertEqual(num_of_events, len(recorded_events), 'wrong number of events')

        for recorded_event_index, recorded_event in enumerate(sorted(recorded_events, key=operator.attrgetter('id'))):
            self.assertEqual(recorded_event_index, recorded_event.id)
            self.assertEqual('e{}'.format(recorded_event_index), self._ensure_str(recorded_event.body))

    # to run memory profiling test, uncomment the tests below
    # and from terminal run with
    # > mprof run python -m py.test test_wrapper.py::TestSubmitEvents::test_memory_profiling_<num> --full-trace
    # and to get its plot use:
    # > mprof plot --backend agg --output <filename>.png
    # def test_memory_profiling_100(self):
    #     self._run_memory_profiling(100)
    #
    # def test_memory_profiling_1k(self):
    #     self._run_memory_profiling(1000)
    #
    # def test_memory_profiling_10k(self):
    #     self._run_memory_profiling(10000)
    #
    # def test_memory_profiling_100k(self):
    #     self._run_memory_profiling(100000)
    #
    # def _run_memory_profiling(self, num_of_events):
    #     import memory_profiler
    #     self._wait_for_socket_creation()
    #     self._wrapper._entrypoint = unittest.mock.MagicMock()
    #     self._wrapper._entrypoint.return_value = {}
    #     events = (
    #         json.loads(nuclio_sdk.Event(_id=str(i), body='e{0}'.format(i)).to_json())
    #         for i in range(num_of_events)
    #     )
    #     threading.Thread(target=self._send_events, args=(events,)).start()
    #     with open('test_memory_profiling_{0}.txt'.format(num_of_events), 'w') as f:
    #         profiled_serve_requests_func = memory_profiler.profile(self._wrapper.serve_requests,
    #                                                                precision=4,
    #                                                                stream=f)
    #         profiled_serve_requests_func(num_requests=num_of_events)
    #     self.assertEqual(num_of_events, self._wrapper._entrypoint.call_count, 'Received unexpected number of events')

    def _send_events(self, events):
        self._wait_for_socket_creation()
        for event in events:
            self._send_event(event)

    def _send_event(self, event):
        if not isinstance(event, dict):
            event = self._event_to_dict(event)

        # event to a msgpack body message
        body = msgpack.Packer().pack(event)

        # big endian body len
        body_len = struct.pack(">I", len(body))

        # first write body length
        self._unix_stream_server._connection_socket.sendall(body_len)

        # then write body content
        self._unix_stream_server._connection_socket.sendall(body)

    def _get_packed_event_body_len(self, event):
        return len(msgpack.Packer().pack(self._event_to_dict(event)))

    def _event_to_dict(self, event):
        return json.loads(event.to_json())

    def _wait_for_socket_creation(self, timeout=10, interval=0.1):

        # wait for socket connection
        while self._unix_stream_server._connection_socket is None and timeout > 0:
            time.sleep(interval)
            timeout -= interval

    def _wait_until_received_messages(self, minimum_messages_length, timeout=10, interval=1):
        while timeout > 0:
            time.sleep(interval)
            current_messages_length = len(self._unix_stream_server._messages)
            if current_messages_length >= minimum_messages_length:
                return
            self._logger.debug_with('Waiting for messages to arrive',
                                    current_messages_length=current_messages_length,
                                    minimum_messages_length=minimum_messages_length)
            timeout -= interval
        raise RuntimeError('Failed waiting for messages')

    def _create_unix_stream_server(self, socket_path):
        unix_stream_server = _SingleConnectionUnixStreamServer(socket_path, _Connection)

        # create a thread and listen forever on server
        self._unix_stream_server_thread = threading.Thread(target=unix_stream_server.serve_forever)
        self._unix_stream_server_thread.daemon = True
        self._unix_stream_server_thread.start()
        return unix_stream_server

    def _ensure_str(self, s, encoding='utf-8', errors='strict'):

        # Optimization: Fast return for the common case.
        if type(s) is str:
            return s
        if isinstance(s, bytes):
            return s.decode(encoding, errors)
        raise TypeError(f"not expecting type '{type(s)}'")

    def _write_handler(self, temp_path):
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


class TestSubmitEventsDecoded(TestSubmitEvents):
    @classmethod
    def setUpClass(cls):
        super(TestSubmitEventsDecoded, cls).setUpClass()
        cls._decode_incoming_event_messages = True


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

                message = {
                    'type': line[0],
                    'body': json.loads(line[1:]) if line[0] != 's' else ''
                }

                self.server._messages.append(message)

            except:
                pass


class TestCallFunction(unittest.TestCase):

    def setUp(self):
        # provided by _connection_provider
        self._mockConnection = unittest.mock.MagicMock()

    def test_call_json_body(self):
        self._platform = nuclio_sdk.Platform('local', 'somens', self._connection_provider)

        # prepare an event to send
        event = nuclio_sdk.Event(method='GET', path='path', body={'a': 'some_body'})

        # prepare a responder
        connection_response = unittest.mock.MagicMock()
        connection_response.status = http.client.NO_CONTENT
        connection_response.getheaders = lambda: [('Content-Type', 'application/json')]
        connection_response.read = unittest.mock.MagicMock(return_value='{"b": "some_response"}')

        self._mockConnection.getresponse = unittest.mock.MagicMock(return_value=connection_response)

        # send the event
        response = self._platform.call_function('function-name', event)

        self.assertEqual(self._mockConnection.url, 'nuclio-somens-function-name:8080')
        self._mockConnection.request.assert_called_with(event.method,
                                                        event.path,
                                                        body=json.dumps({'a': 'some_body'}),
                                                        headers={
                                                            'Content-Type': 'application/json',
                                                            'X-Nuclio-Target': 'function-name'
                                                        })

        self.assertEqual({'b': 'some_response'}, response.body)
        self.assertEqual('application/json', response.content_type)
        self.assertEqual(http.client.NO_CONTENT, response.status_code)

    def test_get_function_url(self):
        self.assertEqual(nuclio_sdk.Platform('local', 'ns')._get_function_url('function-name'),
                         'nuclio-ns-function-name:8080')
        self.assertEqual(nuclio_sdk.Platform('kube', 'ns')._get_function_url('function-name'),
                         'nuclio-function-name:8080')

    def _connection_provider(self, url, timeout=None):
        self._mockConnection.url = url
        return self._mockConnection
