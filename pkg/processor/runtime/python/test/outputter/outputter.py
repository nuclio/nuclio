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
    """Return reversed body as string"""

    body_str = event.body.decode('utf-8')

    if body_str == 'return_string':
        return 'a string'
    elif body_str == 'return_status_and_string':
        return 201, 'a string after status'
    elif body_str == 'return_dict':
        return {'a': 'dict', 'b': 'foo'}
    elif body_str == 'return_status_and_dict':
        return 201, {'a': 'dict after status', 'b': 'foo'}
    elif body_str == 'log':
        context.logger.debug('debug message')
        context.logger.info('info message')
        context.logger.warn('warn message')
        context.logger.error('error message')
