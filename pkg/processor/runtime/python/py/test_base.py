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

import json

import os

import unittest.mock


class BaseTestSubmitEvents(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls._decode_event_strings = False

    def _ensure_str(self, s, encoding='utf-8', errors='strict'):

        # Optimization: Fast return for the common case.
        if type(s) is str:
            return s
        if isinstance(s, bytes):
            return s.decode(encoding, errors)
        raise TypeError(f"not expecting type '{type(s)}'")

    def _event_to_dict(self, event):
        return json.loads(event.to_json())

    def _write_handler(self, temp_path):
        handler_code = '''import sys

def handler(ctx, event):
    """Return reversed body as string"""
    body = event.body
    if isinstance(event.body, bytes):
        body = event.body.decode('utf-8')
    ctx.logger.warn('the end is nigh')
    return body[::-1]
'''

        handler_path = os.path.join(temp_path, 'reverser.py')

        with open(handler_path, 'w') as out:
            out.write(handler_code)

        return handler_path
