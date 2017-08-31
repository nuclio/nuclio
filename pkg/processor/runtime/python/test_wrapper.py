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
import wrapper

import pytest

from base64 import b64encode
from datetime import datetime, timedelta
from email.mime.text import MIMEText  # Use in load_module test
from os import environ
from os.path import abspath, dirname
from subprocess import Popen
from sys import executable
from tempfile import mkdtemp
from threading import Thread, Event
import json
import logging
import sys


is_py3 = sys.version_info[:2] >= (3, 0)
here = dirname(abspath(__file__))
expected_time = datetime(2017, 8, 14, 20, 40, 31, 444845)

if is_py3:
    from datetime import timezone

    tzinfo = timezone(timedelta(0, 10800))
    expected_time = expected_time.replace(tzinfo=tzinfo)
    from socketserver import UnixStreamServer, BaseRequestHandler
    from io import StringIO
else:
    from SocketServer import UnixStreamServer, BaseRequestHandler
    from io import BytesIO as StringIO


payload = b'marry had a little lamb'

test_event = {
  'version': 'version 1',
  'id': 'id 1',
  'source': {
    'class': 'some class',
    'kind': 'some kind',
  },
  'content-type': 'text/plain',
  'body': b64encode(payload).decode('utf-8'),
  'size': 23,
  'headers': {
    'header-1': 'h1',
    'Header-2': 'h2'
  },
  'timestamp': '2017-08-14T20:40:31.444845002+03:00',
  'path': '/api/v1/event',
  'url': 'http://nuclio.com',
}

test_event_msg = json.dumps(test_event)

handler_module = 'reverser'
handler_func = 'handler'
handler_code = '''
import sys

is_py2 = sys.version_info[:2] < (3, 0)


def {}(ctx, event):
    """Return reversed body as string"""
    body = event.body
    if not is_py2:
        body = body.decode('utf-8')
    ctx.logger.warning('the end is nih')
    return body[::-1]
'''.format(handler_func)


def test_parse_datetime():
    ts = '2017-08-14T20:40:31.444845002+03:00'
    assert wrapper.parse_time(ts) == expected_time


def test_load_handler():
    entry_point = 'email.mime.text:MIMEText'
    obj = wrapper.load_handler(entry_point)
    assert obj is MIMEText

    with pytest.raises(ValueError):
        wrapper.load_handler('json')

    with pytest.raises(ImportError):
        wrapper.load_handler('no_such_module:func')

    with pytest.raises(AttributeError):
        wrapper.load_handler('json:no_such_function')


def test_decode_event():
    event = wrapper.decode_event(test_event_msg)
    assert event.body == payload
    # Check that different case works
    assert event.headers['Header-1'] == 'h1'
    assert event.timestamp == expected_time


class RequestHandler(BaseRequestHandler):
    messages = []
    done = Event()

    def handle(self):
        try:
            self._handle()
        finally:
            self.done.set()

    def _handle(self):
        msg = test_event_msg.encode('utf-8') + b'\n'
        self.request.sendall(msg)

        buf = []
        while True:
            chunk = self.request.recv(1024)

            if not chunk:
                return

            i = chunk.find(b'\n')
            if i == -1:
                buf.append(chunk)
                continue

            data = b''.join(buf) + chunk[:i]
            buf = [data[i+1:]]
            obj = json.loads(data)
            self.messages.append(obj)
            if 'handler_output' in obj:
                return


def run_test_server(sock_path):
    srv = UnixStreamServer(sock_path, RequestHandler)
    thr = Thread(target=srv.serve_forever)
    thr.daemon = True
    thr.start()


def test_handler():
    tmp = mkdtemp()
    with open('{}/{}.py'.format(tmp, handler_module), 'w') as out:
        out.write(handler_code)
    env = environ.copy()
    env['PYTHONPATH'] = '{}:{}'.format(tmp, env.get('PYTHONPATH', ''))

    sock_path = '{}/nuclio.sock'.format(tmp)
    run_test_server(sock_path)

    entry_point = '{}:{}'.format(handler_module, handler_func)
    py_file = '{}/wrapper.py'.format(here)
    cmd = [
        executable, py_file,
        '--entry-point', entry_point,
        '--socket-path', sock_path,
    ]
    child = Popen(cmd, env=env)

    try:
        timeout = 3  # In seconds
        if not RequestHandler.done.wait(timeout):
            assert False, 'No reply after {} seconds'.format(timeout)

        assert len(RequestHandler.messages) == 2, 'Bad number of message'
        log = RequestHandler.messages[0]
        assert 'msg' in log, 'No message in log'

        out = RequestHandler.messages[1]['handler_output']
        assert out.encode('utf-8') == payload[::-1], 'Bad output'
    finally:
        child.kill()


def test_create_logger():
    stdout = sys.stdout
    try:
        io = StringIO()
        sys.stdout = io
        level = logging.WARNING
        logger = wrapper.create_logger(level)
        assert logger.level == level, 'bad level'
        logger.error('oops')
        for handler in logger.handlers:
            handler.flush()
        assert io.getvalue(), 'No output'
    finally:
        sys.stdout = stdout
