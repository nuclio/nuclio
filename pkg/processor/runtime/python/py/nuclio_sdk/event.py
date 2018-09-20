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

import sys
import base64
import json
import datetime


class TriggerInfo(object):

    def __init__(self, klass='', kind=''):
        self.klass = klass
        self.kind = kind


class Event(object):

    def __init__(self,
                 body=None,
                 content_type=None,
                 trigger=None,
                 fields=None,
                 headers=None,
                 _id=None,
                 method=None,
                 path=None,
                 size=None,
                 timestamp=None,
                 url=None,
                 _type=None,
                 type_version=None,
                 version=None):
        self.body = body
        self.content_type = content_type
        self.trigger = trigger or TriggerInfo(klass='', kind='')
        self.fields = fields or {}
        self.headers = headers or {}
        self.id = _id
        self.method = method
        self.path = path or '/'
        self.size = size
        self.timestamp = timestamp or 0
        self.url = url
        self.type = _type
        self.type_version = type_version
        self.version = version

    def to_json(self):
        obj = vars(self).copy()
        obj['trigger'] = {
            'class': self.trigger.klass,
            'kind': self.trigger.kind,
        }
        return json.dumps(obj)

    def get_header(self, header_key):
        for key, value in self.headers.items():
            if key.lower() == header_key.lower():
                return value

    @staticmethod
    def from_json(data):
        """Decode event encoded as JSON by processor"""

        parsed_data = json.loads(data)
        trigger = TriggerInfo(
            parsed_data['trigger']['class'],
            parsed_data['trigger']['kind'],
        )

        # extract content type, needed to decode body
        content_type = parsed_data['content_type']

        return Event(body=Event.decode_body(parsed_data['body'], content_type),
                     content_type=content_type,
                     trigger=trigger,
                     fields=parsed_data.get('fields'),
                     headers=parsed_data.get('headers'),
                     _id=parsed_data['id'],
                     method=parsed_data['method'],
                     path=parsed_data['path'],
                     size=parsed_data['size'],
                     timestamp=datetime.datetime.utcfromtimestamp(parsed_data['timestamp']),
                     url=parsed_data['url'],
                     _type=parsed_data['type'],
                     type_version=parsed_data['type_version'],
                     version=parsed_data['version'])

    @staticmethod
    def decode_body(body, content_type):
        """Decode event body"""

        if isinstance(body, dict):
            return body
        else:
            try:
                decoded_body = base64.b64decode(body)
            except:
                return body

            if content_type == 'application/json':
                try:
                    return json.loads(decoded_body)
                except:
                    pass

            return decoded_body
