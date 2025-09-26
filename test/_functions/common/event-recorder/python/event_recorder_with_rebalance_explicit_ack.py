# Copyright 2025 The Nuclio Authors.
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
import os
import nuclio_sdk
import time

events_log_file_path = '/tmp/events.json'


def handler(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    if _ensure_str(event.trigger.kind) in ["kafka-cluster", "v3ioStream", "v3io-stream"]:
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

        # mark offset
        context.last_processed_offsets.set_last_processed_offset(topic=event.topic, partition= event.shard_id, offset=event.offset)

    elif _ensure_str(event.trigger.kind) == 'http':

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
    return None


def _ensure_str(s, encoding='utf-8', errors='strict'):

    # Optimization: Fast return for the common case.
    if type(s) is str:
        return s
    if isinstance(s, bytes):
        return s.decode(encoding, errors)
    raise TypeError(f"not expecting type '{type(s)}'")

def init_context(context):
    context.logger.info_with('Initializing', worker_id=context.worker_id)
    context.last_processed_offsets = LastProcessedOffsets(context)
    # register a callback to be called when the function is drained
    context.platform.set_drain_callback(context.last_processed_offsets.drain)
    context.platform.set_termination_callback(context.last_processed_offsets.drain)

def drain_callback():
    print('Drain callback called!')
    sleep_interval = os.getenv("DRAIN_SLEEP_INTERVAL", "1")
    time.sleep(float(sleep_interval))
    print('Slept for {0} seconds'.format(sleep_interval))

class LastProcessedOffsets:
    def __init__(self, context: nuclio_sdk.Context):
        self.last_processed_offset_of_topic = {}
        self.context = context

    def set_last_processed_offset(self, topic, partition, offset):
        self.last_processed_offset_of_topic[(topic, partition)] = offset


    def drain(self):
        return self._drain()

    async def _drain(self):
        self.context.logger.info("Draining - committing offsets")
        for (topic, partition), offset in self.last_processed_offset_of_topic.items():
            self.context.logger.info(f'Topic: {topic}, Partition: {partition}, Offset: {offset}')
            await self.context.platform.explicit_ack(nuclio_sdk.QualifiedOffset(topic=topic, partition=partition, offset=offset))



