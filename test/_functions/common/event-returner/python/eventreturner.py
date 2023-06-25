# Copyright 2023 The Nuclio Authors.
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

import json


def handler(context, event):

    # for object bodies, just take it as is. otherwise decode
    if not isinstance(event.body, dict):
        body = event.body.decode('utf8')
    else:
        body = event.body

    headers = {
        _ensure_str(header): _ensure_str(value)
        for header, value in event.headers.items()
    }

    return json.dumps({
        'id': event.id,
        'eventType': event.trigger.kind,
        'contentType': event.content_type,
        'headers': headers,
        'timestamp': event.timestamp.isoformat('T') + 'Z',
        'path': event.path,
        'url': event.url,
        'method': event.method,
        'type': event.type,
        'typeVersion': event.type_version,
        'version': event.version,
        'body': body
    }, default=_json_default)


def _json_default(s):
    if type(s) is bytes:
        return _ensure_str(s)
    return s


def _ensure_str(s, encoding='utf-8', errors='strict'):

    # Optimization: Fast return for the common case.
    if type(s) is str:
        return s
    if isinstance(s, bytes):
        return s.decode(encoding, errors)
    raise TypeError(f"not expecting type '{type(s)}'")
