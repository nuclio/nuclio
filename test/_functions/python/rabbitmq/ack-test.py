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
import os
import nuclio_sdk

events_file_path = '/tmp/events.json'


async def handler(context, event):
    context.logger.debug('Received event! event.body: {0}, event.headers: {1}, kind: {2}'.format(event.body, event.headers, event.trigger.kind))

    if event.trigger.kind == "rabbit-mq":
        body = event.body.decode('utf-8')
        if "nack-once" in body and event.body not in context.already_nacked:
            context.logger.info("Simulating failure for event: {}".format(body))
            context.already_nacked.add(event.body)
            raise Exception("Simulated failure")
        write_event_to_file(event)
        response = nuclio_sdk.Response()
        response.status_code = 200
        return response
    else:
        try:
            with open(events_file_path, 'r') as events_log_file:
                events_log_file_contents = events_log_file.read()
        except IOError:
            events_log_file_contents = ''

        # make this valid JSON by removing last two chars (, ) and enclosing in [ ]
        encoded_event_log = '[' + events_log_file_contents[:-2] + ']'

        context.logger.info('Returning events: {0}'.format(encoded_event_log))

        return encoded_event_log



def init_context(context):
    context.logger.info('Initializing context')
    context.already_nacked = set()
    # create file - first line is queue size, second line is last commit offset
    if not os.path.exists(events_file_path):
        with open(events_file_path, 'w'):
            pass


def write_event_to_file(event: nuclio_sdk.Event):

    with open(events_file_path, 'a') as file:
        qualified_offset_json = json.dumps({
            'topic': event.path,
            'body': event.body.decode('utf-8')
        })
        file.write(qualified_offset_json + ', ')
