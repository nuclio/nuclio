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
from contextlib import contextmanager
from datetime import datetime
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
timestamp = 1504261658
expected_time = datetime.utcfromtimestamp(timestamp)

if is_py3:
    from socketserver import UnixStreamServer, BaseRequestHandler
    from io import StringIO
else:
    from SocketServer import UnixStreamServer, BaseRequestHandler
    from io import BytesIO as StringIO
    json.JSONDecodeError = ValueError


payload = b'marry had a little lamb'

test_event = {
    'body': b64encode(payload).decode('utf-8'),
    'content-type': 'text/plain',
    'trigger': {
        'class': 'some class',
        'kind': 'some kind',
    },
    'fields': {
        'fields-1': 'f1',
        'fields-2': 'f2',
    },
    'headers': {
        'header-1': 'h1',
        'Header-2': 'h2'
    },
    'id': 'id 1',
    'method': 'POST',
    'path': '/api/v1/event',
    'size': 23,
    'timestamp': timestamp,
    'url': 'http://nuclio.com',
    'version': 'version 1',
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


def test_load_handler():
    handler = 'email.mime.text:MIMEText'
    obj = wrapper.load_handler(handler)
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

        fp = self.request.makefile('r')

        while True:
            line = fp.readline()
            if not line:
                break

            typ, msg = line[0], json.loads(line[1:])
            self.messages.append(msg)

            if typ == 'r':  # reply
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

    handler = '{}:{}'.format(handler_module, handler_func)
    py_file = '{}/wrapper.py'.format(here)
    cmd = [
        executable, py_file,
        '--handler', handler,
        '--socket-path', sock_path,
    ]
    child = Popen(cmd, env=env)

    try:
        timeout = 3  # In seconds
        if not RequestHandler.done.wait(timeout):
            assert False, 'No reply after {} seconds'.format(timeout)

        assert len(RequestHandler.messages) == 3, 'Bad number of message'
        log = RequestHandler.messages[0]
        assert 'message' in log, 'No message in log'

        metric = RequestHandler.messages[1]
        assert 'duration' in metric, 'No duration in metric'

        out = RequestHandler.messages[-1]['body']
        assert out.encode('utf-8') == payload[::-1], 'Bad output'
    finally:
        child.kill()


@contextmanager
def make_logger(level=logging.ERROR):
    io = StringIO()
    stdout = sys.stdout
    sys.stdout = io
    logger = wrapper.create_logger(level)
    try:
        yield logger, io
    finally:
        for handler in logger.handlers:
            handler.flush()
        sys.stdout = stdout


def test_create_logger():
    level = logging.WARNING
    with make_logger(level) as (logger, io):
        assert logger.level == level, 'bad level'
        logger.error('oops')

    assert io.getvalue(), 'No output'


def test_log():
    message = 'how are you?'

    with make_logger() as (logger, io):
        logger.error(message)

    out = io.getvalue()
    assert out[0] == 'l', 'bad prefix'

    record = json.loads(out[1:])
    assert record['message'] == message, 'bad message'
    assert record['level'] == 'error', 'bad level'
    assert record.get('datetime'), 'no timestamp'
    assert record['with'] == {}, 'bad with'


def test_log_with():
    message = 'whats up with?'
    kw = {'source': 'rabbit', 'duration': 7.3}

    with make_logger() as (logger, io):
        logger.error(message)

    with make_logger(logging.INFO) as (logger, io):
        logger.info_with(message, **kw)

    out = io.getvalue()
    assert out[0] == 'l', 'bad prefix'

    record = json.loads(out[1:])
    assert record['message'] == message, 'bad message'
    assert record['level'] == 'info', 'bad level'
    assert record.get('datetime'), 'no timestamp'
    assert record['with'] == kw, 'bad with'
