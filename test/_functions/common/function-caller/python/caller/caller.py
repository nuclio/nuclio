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

def handler(context, event):

    # modify the event body
    event.body['caller_body_value'] = 'caller_body'

    # modify event headers
    event.headers = {
        'x-caller-header-value': 'caller_header'
    }

    # modify method
    event.method = 'PUT'

    # modify path
    event.path = '/caller/path'

    # return the response from the called function
    return context.platform.call_function(event.body['callee_name'], event, timeout=5)
