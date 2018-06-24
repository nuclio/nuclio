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

import os
import time
import json
import re


def parse_duration(duration):
    """Parse duration in '2.3s' format to float (seconds)"""
    # '10ms', '2.3s', ...
    match = re.match('(\d+(\.\d+)?)([a-z]+)', duration)
    if not match:
        return None

    amount = float(match.group(1))
    unit = {
        'h': 60 * 60,
        'm': 60,
        's': 1,
        'ms': 0.001,
        'ns': 0.000001,
        'us': 0.000000001,
    }.get(match.group(3))

    if unit is None:
        return None

    return amount * unit


def handler(context, event):
    """Wait a timeout amount and return current PID"""
    body = event.body

    context.logger.debug('Event body: %r', body)
    if isinstance(body, bytes):
        body = json.loads(body)

    timeout = parse_duration(body['timeout'])
    if timeout is None:
        context.logger.error('bad timeout: %r', event.body)
        return json.dumps({'error': 'bad timeout'})

    context.logger.info('Sleeping %.3f seconds', timeout)
    time.sleep(timeout)
    return json.dumps({'pid': os.getpid()})
