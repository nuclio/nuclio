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
from time import time
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

TriggerInfo = namedtuple('TriggerInfo', ['klass',  'kind'])
Event = namedtuple(
    'Event', [
        'body',
        'content_type',
        'trigger',
        'fields',
        'headers',
        'id',
        'method',
        'path',
        'size',
        'timestamp',
        'url',
        'version',
    ],
)

Response = namedtuple(
    'Response', [
        'headers',
        'body',
        'content_type',
        'status_code',
    ],
)

# TODO: data_binding
Context = namedtuple('Context', ['logger', 'data_binding', 'Response'])


class JSONEncoder(json.JSONEncoder):
    """JSON encoder that can encode Headers"""
    def default(self, obj):
        if isinstance(obj, Headers):
            return dict(obj)
        # Let the base class default method raise the TypeError
        return json.JSONEncoder.default(self, obj)


json_encode = JSONEncoder().encode


def create_logger(level=logging.DEBUG):
    """Create a logger that emits JSON to stdout"""
    logger = logging.getLogger()
    logger.setLevel(level)

    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(JSONFormatter())
    logger.addHandler(handler)

    # Add info_with and friends to logger
    for name in ['critical', 'fatal', 'error', 'warning', 'info', 'debug']:
        add_structured_log_method(logger, name)

    return logger


def decode_body(body):
    """Decode event body"""
    return b64decode(body)


def decode_event(data):
    """Decode event encoded as JSON by Go"""
    obj = json.loads(data)
    trigger = TriggerInfo(
        obj['trigger']['class'],
        obj['trigger']['kind'],
    )

    # Headers are case insensitive
    headers = Headers()
    obj_headers = obj['headers'] or {}
    for key, value in obj_headers.items():
        headers[key] = value

    return Event(
        body=decode_body(obj['body']),
        content_type=obj['content-type'],
        trigger=trigger,
        fields=obj.get('fields') or {},
        headers=headers,
        id=obj['id'],
        method=obj['method'],
        path=obj['path'],
        size=obj['size'],
        timestamp=datetime.utcfromtimestamp(obj['timestamp']),
        url=obj['url'],
        version=obj['version'],
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
    match = re.match('^([\w|-]+(\.[\w|-]+)*):(\w+)$', handler)
    if not match:
        raise ValueError('malformed handler')

    mod_name, func_name = match.group(1), match.group(3)
    mod = load_module(mod_name)

    return getattr(mod, func_name)


# Logging support
class JSONFormatter(logging.Formatter):
    def format(self, record):
        record_fields = {
            'datetime': self.formatTime(record, self.datefmt),
            'level': record.levelname.lower(),
            'message': record.getMessage(),
            'with': getattr(record, 'with', {}),
        }

        return 'l' + json_encode(record_fields)


def add_structured_log_method(logger, name):
    """Add a `<name>_with` method to logger.

    This will populate the `extra` parameter with `with` key
    """
    method = getattr(logger, name)

    def with_method(message, *args, **kw):
        method(message, *args, extra={'with': kw})

    setattr(logger, '{}_with'.format(name), with_method)


def serve_requests(sock, logger, handler):
    """Read event from socket, send out reply"""

    buf = []
    ctx = Context(logger, None, Response)
    stream = sock.makefile('w')

    while True:

        formatted_exception = None

        try:

            # try to read a packet (delimited by \n) from the wire
            packet = get_next_packet(sock, buf)

            # we could've received partial data. read more in this case
            if packet is None:
                continue

            # decode the JSON encoded event
            event = decode_event(packet)

            # returned result
            handler_output = ''

            try:
                start_time = time()
                handler_output = handler(ctx, event)
                duration = time() - start_time

                stream.write('m' + json.dumps({'duration': duration}) + '\n')
                stream.flush()

                response = response_from_handler_output(handler_output)

                # try to json encode the response
                encoded_response = json_encode(response)

            except Exception as err:
                formatted_exception = \
                    'Exception caught in handler "{0}": {1}'.format(
                        err, traceback.format_exc())

        except Exception as err:
            formatted_exception = \
                'Exception caught while serving "{0}": {1}'.format(
                    err, traceback.format_exc())

        # if we have a formatted exception, return it as 500
        if formatted_exception is not None:
            logger.warn(formatted_exception)

            encoded_response = json_encode({
                'status_code': 500,
                'content_type': 'text/plain',
                'body': formatted_exception,
            })

        # write to the socket
        stream.write('r' + encoded_response + '\n')
        stream.flush()


def get_next_packet(sock, buf):
    chunk = sock.recv(1024)

    if not chunk:
        raise RuntimeError('Failed to read from socket (empty chunk)')

    i = chunk.find(b'\n')
    if i == -1:
        buf.append(chunk)
        return None

    packet = b''.join(buf) + chunk[:i]
    buf = [packet[i+1:]]

    return packet


def response_from_handler_output(handler_output):
    """Given a handler output's type, generates a response towards the
    processor"""

    response = {
        'body': '',
        'content_type': 'text/plain',
        'headers': {},
        'status_code': 200,
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
            response['body'] = json_encode(handler_output[1])
            response['content_type'] = 'application/json'

    # if it's a dict, populate the response and set content type to json
    elif type(handler_output) is dict or type(handler_output) is list:
        response['content_type'] = 'application/json'
        response['body'] = json_encode(handler_output)

    # if it's a response object, populate the response
    elif type(handler_output) is Response:
        response['body'] = handler_output.body
        response['content_type'] = handler_output.content_type
        response['headers'] = handler_output.headers
        response['status_code'] = handler_output.status_code
    else:
        response['body'] = handler_output

    return response


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

        serve_requests(sock, logger, event_handler)

    except Exception as err:
        logger.warning(
            'Caught unhandled exception while initializing "{0}": {1}'.format(
             err, traceback.format_exc()))
        raise SystemExit(1)


if __name__ == '__main__':
    main()
