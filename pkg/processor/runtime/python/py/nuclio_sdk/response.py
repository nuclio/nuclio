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

import base64
import json


class Response(object):

    def __init__(self, headers=None, body=None, content_type=None, status_code=200):
        self.headers = headers
        self.body = body
        self.status_code = status_code
        self.content_type = content_type

    def __repr__(self):
        cls = self.__class__.__name__
        items = self.__dict__.items()
        args = ('{}={!r}'.format(key, value) for key, value in items)
        return '{}({})'.format(cls, ', '.join(args))

    @staticmethod
    def from_entrypoint_output(json_encoder, handler_output):
        """Given a handler output's type, generates a response towards the
        processor"""

        response = {
            'body': '',
            'content_type': 'text/plain',
            'headers': {},
            'status_code': 200,
            'body_encoding': 'text',
        }

        # if the type of the output is a string, just return that and 200
        if isinstance(handler_output, str):
            response['body'] = handler_output

        # if it's a tuple of 2 elements, first is status second is body
        elif isinstance(handler_output, tuple) and len(handler_output) == 2:
            response['status_code'] = handler_output[0]

            if isinstance(handler_output[1], str):
                response['body'] = handler_output[1]
            else:
                response['body'] = json_encoder(handler_output[1])
                response['content_type'] = 'application/json'

        # if it's a dict, populate the response and set content type to json
        elif isinstance(handler_output, dict) or isinstance(handler_output, list):
            response['content_type'] = 'application/json'
            response['body'] = json_encoder(handler_output)

        # if it's a response object, populate the response
        elif isinstance(handler_output, Response):
            if isinstance(handler_output.body, dict):
                response['body'] = json.dumps(handler_output.body)
                response['content_type'] = 'application/json'
            else:
                response['body'] = handler_output.body
                response['content_type'] = handler_output.content_type

            response['headers'] = handler_output.headers
            response['status_code'] = handler_output.status_code
        else:
            response['body'] = handler_output

        if isinstance(response['body'], bytes):
            response['body'] = base64.b64encode(response['body']).decode('ascii')
            response['body_encoding'] = 'base64'

        return response
