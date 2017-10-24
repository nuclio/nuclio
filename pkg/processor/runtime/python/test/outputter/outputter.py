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
    """Given a certain body, returns a response. Used by an integration test"""

    # if the method is other than POST, return it as the body
    if event.method != 'POST':
        return event.method

    body_str = event.body.decode('utf-8')

    if body_str == 'return_string':
        return 'a string'
    elif body_str == 'return_status_and_string':
        return 201, 'a string after status'
    elif body_str == 'return_dict':
        return {'a': 'dict', 'b': 'foo'}
    elif body_str == 'return_list':
        return [{'a': 1}, {'b': 2}]
    elif body_str == 'return_status_and_dict':
        return 201, {'a': 'dict after status', 'b': 'foo'}
    elif body_str == 'log':
        context.logger.debug('Debug message')
        context.logger.info('Info message')
        context.logger.warn('Warn message')
        context.logger.error('Error message')

        return 201, 'returned logs'

    elif body_str == 'log_with':
        context.logger.error_with(
            'Error message', source='rabbit', weight=7)
        return 201, 'returned logs with'

    elif body_str == 'return_response':

        # echo back the headers, plus add two (TODO)
        headers = event.headers
        headers['h1'] = 'v1'
        headers['h2'] = 'v2'

        return context.Response(
            body='response body',
            headers=headers,
            content_type='text/plain',
            status_code=201)

    elif body_str == 'return_fields':
        # We use sorted to get predictable output
        kvs = ['{}={}'.format(k, v) for k, v in sorted(event.fields.items())]
        return ','.join(kvs)

    elif body_str == 'return_path':
        return event.path

    elif body_str == 'return_error':
        raise ValueError('some error')

    else:
        raise RuntimeError('Unknown return mode: {0}'.format(body_str))
