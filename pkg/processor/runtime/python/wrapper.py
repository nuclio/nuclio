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
import traceback
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

EventSourceInfo = namedtuple('EventSourceInfo', ['klass',  'kind'])
Event = namedtuple(
    'Event', [
        'version',
        'id',
        'event_source',
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


def decode_body(body):
    """Decode event body"""
    return b64decode(body)


def decode_event(data):
    """Decode event encoded as JSON by Go"""
    obj = json.loads(data)
    event_source = EventSourceInfo(obj['event_source']['class'], obj['event_source']['kind'])

    # Headers are insensitive
    headers = Headers()
    obj_headers = obj['headers'] or {}
    for key, value in obj_headers.items():
        headers[key] = value

    return Event(
        version=obj['version'],
        id=obj['id'],
        event_source=event_source,
        content_type=obj['content-type'],
        body=decode_body(obj['body']),
        size=obj['size'],
        headers=headers,
        timestamp=datetime.utcfromtimestamp(obj['timestamp']),
        path=obj['path'],
        url=obj['url'],
    )


def load_module(name):
    """Load module in the format 'json.tool'"""
    mod = __import__(name)
    for sub in name.split('.')[1:]:
        mod = getattr(mod, sub)
    return mod


def load_handler(handler):
    """Load handler function from handler.

    handler is in the format 'module.sub:handler_name'
    """
    match = re.match('^(\w+(\.\w+)*):(\w+)$', handler)
    if not match:
        raise ValueError('malformed handler')

    mod_name, func_name = match.group(1), match.group(3)
    mod = load_module(mod_name)

    return getattr(mod, func_name)


# Logging support
class JSONFormatter(logging.Formatter):
    def format(self, record):
        record_fields = {
            'message': record.msg,
            'level': record.levelname.lower(),
            'datetime': self.formatTime(record, self.datefmt)
        }

        return 'l' + json.dumps(record_fields)


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

        stream = sock.makefile('w')

        # returned result
        handler_output = ''

        try:
            handler_output = handler(ctx, event)
        except Exception as e:
            logger.warn('Exception caught in handler "{0}": {1}'.format(e, traceback.format_exc()))

        response = {
            'status_code': 200,
            'content_type': 'text/plain',
            'body': ''
        }

        # if the type of the output is a string, just return that and 200
        if type(handler_output) is str:
            response['body'] = handler_output

        # if it's a tuple of 2 elements, first is status second is body
        elif type(handler_output) is tuple and len(handler_output) == 2:
            response['status_code'] = handler_output[0]

            if type(handler_output[1]) is str:
                response['body'] = handler_output[1]
            else:
                response['body'] = json.dumps(handler_output[1])
                response['content_type'] = 'application/json'

        # if it's a dict, populate the response
        elif type(handler_output) is dict:
            response = handler_output

        # write to the socket
        stream.write('r' + json.dumps(response))

        stream.write('\n')
        stream.flush()

def add_socket_handler_to_logger(logger, sock):
    """Add a handler that will write log message to socket"""
    handler = logging.StreamHandler(sock.makefile('w'))
    handler.setFormatter(JSONFormatter())
    logger.addHandler(handler)


def main():
    from argparse import ArgumentParser

    parser = ArgumentParser(description=__doc__)
    parser.add_argument(
        '--handler', help='handler (module.sub:handler)',
        required=True)
    parser.add_argument(
        '--socket-path', help='path to unix socket to listen on',
        required=True)
    args = parser.parse_args()

    logger = create_logger()
    try:
        logger.debug('args={}'.format(vars(args)))

        event_handler = load_handler(args.handler)

        sock = socket(AF_UNIX, SOCK_STREAM)
        sock.connect(args.socket_path)

        add_socket_handler_to_logger(logger, sock)
        serve_forever(sock, logger, event_handler)

    except Exception as e:
        logger.warn('Caught unhandled exception "{0}": {1}'.format(e, traceback.format_exc()))
        raise SystemExit(1)


if __name__ == '__main__':
    main()
