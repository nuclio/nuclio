import wrapper

import pytest

from base64 import b64encode
from datetime import datetime, timedelta
from email.mime.text import MIMEText  # Use in load_module test
from os import environ
from os.path import abspath, dirname
from subprocess import Popen, PIPE
from sys import executable
from tempfile import mkdtemp
import json
import sys


is_py3 = sys.version_info[:2] >= (3, 0)
here = dirname(abspath(__file__))
expected_time = datetime(2017, 8, 14, 20, 40, 31, 444845)

if is_py3:
    from datetime import timezone

    tzinfo = timezone(timedelta(0, 10800))
    expected_time = expected_time.replace(tzinfo=tzinfo)

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


def {}(event):
    """Return reversed body as string"""
    body = event.body
    if not is_py2:
        body = body.decode('utf-8')
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


def test_handler():
    tmp = mkdtemp()
    with open('{}/{}.py'.format(tmp, handler_module), 'w') as out:
        out.write(handler_code)
    env = environ.copy()
    env['PYTHONPATH'] = '{}:{}'.format(tmp, env.get('PYTHONPATH', ''))

    entry_point = '{}:{}'.format(handler_module, handler_func)
    py_file = '{}/wrapper.py'.format(here)
    cmd = [executable, py_file, entry_point]
    child = Popen(cmd, env=env, stdin=PIPE, stdout=PIPE)
    child.stdin.write(test_event_msg.encode('utf-8'))
    child.stdin.close()

    out = child.stdout.read()
    assert out == payload[::-1]
