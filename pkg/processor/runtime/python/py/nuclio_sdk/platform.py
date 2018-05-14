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
import sys

# different HTTP client libraries for Python 2/3
if sys.version_info[:2] < (3, 0):
    from httplib import HTTPConnection
else:
    from http.client import HTTPConnection

import nuclio_sdk


class Platform(object):

    def __init__(self, kind, namespace='default', connection_provider=None):
        self.kind = kind
        self.namespace = namespace

        # connection_provider is used for unit testing
        self._connection_provider = connection_provider or HTTPConnection

    def call_function(self, function_name, event, node=None):

        # get connection from provider
        connection = self._connection_provider(self._get_function_url(function_name))

        # if the user passes a dict as a body, assume json serialization. otherwise take content type from
        # body or use plain text
        if isinstance(event.body, dict):
            body = json.dumps(event.body)
            content_type = 'application/json'
        else:
            body = event.body
            content_type = event.content_type or 'text/plain'

        connection.request(event.method,
                           event.path,
                           body=body,
                           headers={'Content-Type': content_type})

        # get response from connection
        connection_response = connection.getresponse()

        # header dict
        response_headers = {}

        # get response headers as lowercase
        for (name, value) in connection_response.getheaders():
            response_headers[name.lower()] = value

        # if content type exists, use it
        response_content_type = response_headers.get('content-type', 'text/plain')

        # read the body
        response_body = connection_response.read()

        # if content type is json, go ahead and do parsing here. if it explodes, don't blow up
        if response_content_type == 'application/json':
            response_body = json.loads(response_body)

        response = nuclio_sdk.Response(headers=response_headers,
                                       body=response_body,
                                       content_type=response_content_type,
                                       status_code=connection_response.status)

        return response

    def _get_function_url(self, function_name):

        # local envs prefix namespace
        if self.kind == 'local':
            return '{0}-{1}:8080'.format(self.namespace, function_name)
        else:
            return '{0}:8080'.format(function_name)
