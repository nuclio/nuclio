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

import json

def handler(context, event):

    # for object bodies, just take it as is. otherwise decode
    if not isinstance(event.body, dict):
        body = event.body.decode('utf8')
    else:
        body = event.body

    return json.dumps({
        'id': event.id,
        'triggerClass': event.trigger.klass,
        'eventType': event.trigger.kind,
        'contentType': event.content_type,
        'headers': dict(event.headers),
        'timestamp': event.timestamp.isoformat('T') + 'Z',
        'path': event.path,
        'url': event.url,
        'method': event.method,
        'type': event.type,
        'typeVersion': event.type_version,
        'version': event.version,
        'body': body
    })
