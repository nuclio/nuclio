"""Nuclio event handler"""
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

from base64 import b64decode
from collections import namedtuple
from datetime import datetime
from socket import socket, AF_UNIX, SOCK_STREAM
import json
import logging
import re
import sys

is_py2 = sys.version_info[:2] < (3, 0)

if is_py2:
    from httplib import HTTPMessage
    from io import BytesIO

    class Headers(HTTPMessage):
        def __init__(self):
            HTTPMessage.__init__(self, BytesIO())

else:
    from http.client import HTTPMessage as Headers

SourceInfo = namedtuple('SourceInfo', ['klass',  'kind'])
Event = namedtuple(
    'Event', [
        'version',
        'id',
        'source',
        'content_type',
        'body',
        'size',
        'headers',
        'timestamp',
        'path',
        'url',
    ],
)

# TODO: data_binding
Context = namedtuple('Context', ['logger', 'data_binding'])


def create_logger(level=logging.DEBUG):
    """Create a logger that emits JSON to stdout"""
    logger = logging.getLogger()
    logger.setLevel(level)

    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(JSONFormatter())
    logger.addHandler(handler)

    return logger


def parse_time(data):
    """Parse Go formatted time"""
    if data == '0001-01-01T00:00:00Z':
        return datetime.min

    # Remove ns and change +03:00 to +0300
    data = re.sub(r'\d{3}([+-]\d{2}):(\d{2})', r'\1\2', data)
    if is_py2:
        # No %z (time zone) in Python 2
        return datetime.strptime(data[:-5], '%Y-%m-%dT%H:%M:%S.%f')
    else:
        return datetime.strptime(data, '%Y-%m-%dT%H:%M:%S.%f%z')


def parse_body(body):
    """Parse event body"""
    return b64decode(body)


def decode_event(data):
    """Decode event encoded as JSON by Go"""
    obj = json.loads(data)
    source = SourceInfo(obj['source']['class'], obj['source']['kind'])

    # Headers are insensitive
    headers = Headers()
    obj_headers = obj['headers'] or {}
    for key, value in obj_headers.items():
        headers[key] = value

    return Event(
        version=obj['version'],
        id=obj['id'],
        source=source,
        content_type=obj['content-type'],
        body=parse_body(obj['body']),
        size=obj['size'],
        headers=headers,
        timestamp=parse_time(obj['timestamp']),
        path=obj['path'],
        url=obj['url'],
    )


def load_module(name):
    """Load module in the format 'json.tool'"""
    mod = __import__(name)
    for sub in name.split('.')[1:]:
        mod = getattr(mod, sub)
    return mod


def load_handler(entry_point):
    """Load handler function from entry point.

    entry_point is in the format 'module.sub:handler_name'
    """
    match = re.match('^(\w+(\.\w+)*):(\w+)$', entry_point)
    if not match:
        raise ValueError('maleformed entry point')
    mod_name, func_name = match.group(1), match.group(3)
    mod = load_module(mod_name)
    return getattr(mod, func_name)


# Logging support
class JSONFormatter(logging.Formatter):
    def format(self, record):
        obj = vars(record)

        # Convert to JSON compatible types
        try:
            json.dumps(obj['msg'])
        except TypeError:
            obj['msg'] = repr(obj['msg'])

        if obj['exc_info']:
            obj['exc_info'] = self.formatException(obj['exc_info'])

        return json.dumps(obj)


def serve_forever(sock, logger, handler):
    """Read event from socket, send out reply"""
    buf = []
    ctx = Context(logger, None)

    while True:
        chunk = sock.recv(1024)

        if not chunk:
            return

        i = chunk.find(b'\n')
        if i == -1:
            buf.append(chunk)
            continue
        data = b''.join(buf) + chunk[:i]
        buf = [data[i+1:]]

        if data == b'ping':
            sock.sendall(b'pong\n')
            continue

        event = decode_event(data)
        out = handler(ctx, event)
        # TODO: Handler custom output types (bytes ...)
        reply = json.dumps({'handler_output': out})

        stream = sock.makefile('w')
        stream.write(reply)
        stream.write('\n')
        stream.flush()


def add_sock_handler(logger, sock):
    """Add a handler that will write log message to socket"""
    handler = logging.StreamHandler(sock.makefile('w'))
    handler.setFormatter(JSONFormatter())
    logger.addHandler(handler)


def main():
    from argparse import ArgumentParser

    parser = ArgumentParser(description=__doc__)
    parser.add_argument(
        '--entry-point', help='entry point (module.sub:handler)',
        required=True)
    parser.add_argument(
        '--socket-path', help='path to unix socket to listen on',
        required=True)
    args = parser.parse_args()

    logger = create_logger()
    try:
        logger.debug('args={}'.format(vars(args)))

        event_handler = load_handler(args.entry_point)

        sock = socket(AF_UNIX, SOCK_STREAM)
        sock.connect(args.socket_path)

        add_sock_handler(logger, sock)
        serve_forever(sock, logger, event_handler)

    except Exception as err:
        logger.exception('unhandled exception - {}'.format(err))
        raise SystemExit(1)


if __name__ == '__main__':
    main()
