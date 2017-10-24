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

# @nuclio.configure
#
# processor.yaml:
#   function:
#     kind: "python"
#     python_version: "3"
#     handler: parser:handler
#   triggers:
#     parser_http:
#       class: "sync"
#       kind: "http"
#       enabled: true
#       listen_address: ":7979"
#       num_workers: 4
#
# build.yaml:
#   commands:
#     - pip install simplejson
#

import json

def handler(context, event):
    """Return a field from within a json"""

    try:
        decoded_body = event.body.decode('utf-8')
    except:
        return 400, ""

    context.logger.info('Parsing {0}'.format(decoded_body))

    body = json.loads(decoded_body)

    return body['return_this']
