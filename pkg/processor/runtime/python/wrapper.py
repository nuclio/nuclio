"""Nuclio event handler"""

from base64 import b64decode
from collections import namedtuple
from datetime import datetime
from http.client import HTTPMessage
import json
import re

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


def parse_time(data):
    """Parse Go formatted time"""
    if data == '0001-01-01T00:00:00Z':
        return datetime.min

    # Remove ns and change +03:00 to +0300
    data = re.sub(r'\d{3}([+-]\d{2}):(\d{2})', r'\1\2', data)
    return datetime.strptime(data, '%Y-%m-%dT%H:%M:%S.%f%z')


def decode_event(data):
    """Decode event encoded as JSON by Go"""
    obj = json.loads(data)
    source = SourceInfo(obj['source']['class'], obj['source']['kind'])

    # Headers are insensitive
    headers = HTTPMessage()
    obj_headers = obj['headers'] or {}
    for key, value in obj_headers.items():
        headers[key] = value

    return Event(
        version=obj['version'],
        id=obj['id'],
        source=source,
        content_type=obj['content-type'],
        body=b64decode(obj['body']),
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


def main():
    from argparse import ArgumentParser
    from sys import stdin, stdout

    parser = ArgumentParser(description=__doc__)
    parser.add_argument('entry_point', help='entry point (module.sub:handler)')
    args = parser.parse_args()

    handler = load_handler(args.entry_point)
    data = stdin.read()
    event = decode_event(data)
    out = handler(event)
    stdout.write(out)


if __name__ == '__main__':
    try:
        main()
    except Exception as err:
        with open('/tmp/err.log', 'w') as out:
            out.write(str(err))
