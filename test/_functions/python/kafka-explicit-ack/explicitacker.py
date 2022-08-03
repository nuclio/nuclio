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

"""
    This function simulates usage of the explicit ack feature for kafka triggered functions.
    When it receives events from the kafka cluster, it writes them to a file and sends a 'no-ack' response.
    When the function is triggered by a http request, depending on the request it can:
    1. read the file and return the number of events queued for processing.
    2. read the file and return the last committed offset.
    3. start processing the events in the file - ack each of the events explicitly.
"""

import json
import os
import nuclio_sdk

events_file_path = '/tmp/events.json'
last_committed_offset_file_path = '/tmp/offset.txt'


async def handler(context, event):

    context.logger.debug('Received event! event.body: {0}, event.headers: {1}'.format(event.body, event.headers))

    if event.trigger.kind == 'kafka-cluster':

        context.logger.debug('Adding event to queue - event.body: {0}, event.offset: {1}'.format(event.body,
                                                                                                 event.offset))

        # add event to file
        write_event_to_file(context, event)

        # ensure no ack on response
        response = nuclio_sdk.Response()
        response.status_code = 200
        response.ensure_no_ack()

        return response

    if event.trigger.kind == 'http':

        response = nuclio_sdk.Response()
        response.status_code = 200
        response_body = {}

        resource = json.loads(event.body).get('resource')

        if resource == 'queue_size':
            queue_size = get_number_of_enqueued_events()
            context.logger.info('current queue size: {0}'.format(queue_size))
            response_body['queue_size'] = queue_size

        elif resource == 'last_committed_offset':
            last_commit_offset = get_last_commit_offset()
            context.logger.info('last committed offset: {0}'.format(last_commit_offset))
            response_body['last_committed_offset'] = last_commit_offset

        elif resource == 'start_processing':
            context.logger.info('start processing')
            await process_events(context)
            response_body['started_processing'] = True

        if not resource:
            response_body['not_implemented'] = True
            response.status_code = 400

        response.body = response_body
        return response


def init_context(context):
    context.logger.info('Initializing context')

    # create file - first line is queue size, second line is last commit offset
    if not os.path.exists(events_file_path):
        with open(events_file_path, 'w'):
            pass

    if not os.path.exists(last_committed_offset_file_path):
        with open(last_committed_offset_file_path, 'w'):
            pass


def get_last_commit_offset():
    with open(last_committed_offset_file_path, 'r') as file:
        events = file.readlines()
        return events[0] if len(events) > 0 else str(0)


def get_number_of_enqueued_events():
    with open(events_file_path, 'r') as file:
        events = file.readlines()
        return len(events)


def write_event_to_file(context, event):

    # add event offset to end of file
    offset = event.body.decode('utf-8').split('-')[-1]

    with open(events_file_path, 'a') as file:
        event_json = json.dumps({
            'topic': event.path,
            'partition': event.shard_id,
            'offset': int(offset),
            'trigger_name': event.trigger.name,
            'body': event.body.decode('utf-8')
        })
        file.write(event_json + '\n')


def event_attributes_to_event(event):
    event_attributes = json.loads(event)
    return nuclio_sdk.Event(
        body=event_attributes['body'].encode('utf-8'),
        path=event_attributes['topic'],
        shard_id=event_attributes['partition'],
        offset=event_attributes['offset'],
        trigger=nuclio_sdk.TriggerInfo(
            name=event_attributes['trigger_name'],
            kind='kafka-cluster',
        )
    )


async def process_events(context):
    context.logger.info('Processing events')

    last_commit_offset = 0

    with open(events_file_path, 'r') as file:
        events = file.readlines()
        context.logger.info('Number of events left: {0}'.format(len(events)))
        for event in events:
            event_to_ack = event_attributes_to_event(event)
            context.logger.info('Processing event - body: {0}, offset: {1}'.format(event_to_ack.body,
                                                                                   event_to_ack.offset))
            last_commit_offset = event_to_ack.offset
            await context.platform.explicit_ack(event_to_ack)

    # clear file
    with open(events_file_path, 'w'):
        pass

    # write last committed offset
    with open(last_committed_offset_file_path, 'w') as file:
        file.write(str(last_commit_offset))
