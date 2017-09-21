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
# build.yaml:
#   commands:
#     - pip install requests
#

import json

events_log_file_path = '/tmp/events.json'

def post_event(context, event):
    """post event to the request recorder"""

    context.logger.info('Received event: {0}'.format(event))

    if event.event_source.klass == 'async' and event.event_source.kind == 'rabbitMq':
        body = event.body.decode('utf-8')

        # store in log file
        with open(events_log_file_path, 'a') as events_log_file:
            events_log_file.write(body + ', ')

    else:

        # read the log file
        with open(events_log_file_path, 'r') as events_log_file:
            events_log_file_contents = events_log_file.read()

        encoded_event_log = '[' + events_log_file_contents[:-2] + ']'

        context.logger.info('Returning events: {0}'.format(encoded_event_log))

        # make this valie JSON by removing last two chars (, ) and encloding in [ ]
        return json.loads(encoded_event_log)
