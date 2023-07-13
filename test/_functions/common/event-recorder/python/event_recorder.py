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

import datetime
import json

events_log_file_path = '/tmp/events.json'


def handler(context, event):
    """post event to the request recorder"""

    if _ensure_str(event.trigger.kind) != 'http' or _invoked_by_cron(event):
        body = event.body.decode('utf-8')
        context.logger.info('Received event body: {0}'.format(body))

        # serialized record
        serialized_record = json.dumps({
            'body': body,
            'headers': {
                _ensure_str(header): _ensure_str(value)
                for header, value in event.headers.items()
            },
            'timestamp': datetime.datetime.utcnow().isoformat(),
        })

        # store in log file
        with open(events_log_file_path, 'a') as events_log_file:
            events_log_file.write(serialized_record + ', ')

    else:

        # read the log file
        try:
            with open(events_log_file_path, 'r') as events_log_file:
                events_log_file_contents = events_log_file.read()
        except IOError:
            events_log_file_contents = ''

        # make this valid JSON by removing last two chars (, ) and enclosing in [ ]
        encoded_event_log = '[' + events_log_file_contents[:-2] + ']'

        context.logger.info('Returning events: {0}'.format(encoded_event_log))

        # return json.loads(encoded_event_log)
        return encoded_event_log


def _invoked_by_cron(event):
    return event.get_header('x-nuclio-invoke-trigger') == 'cron' \
           or event.get_header(b'x-nuclio-invoke-trigger') == b'cron'


def _ensure_str(s, encoding='utf-8', errors='strict'):

    # Optimization: Fast return for the common case.
    if type(s) is str:
        return s
    if isinstance(s, bytes):
        return s.decode(encoding, errors)
    raise TypeError(f"not expecting type '{type(s)}'")
