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

from find_handlers import find_handlers

import pytest

from os import path

here = path.dirname(path.abspath(__file__))
test_cases_dir = f'{here}/test/find_handlers'

test_cases = [
    ('sig.py', [('sig', 'handler_a'), ('sig', 'handler_b')]),
    ('dec.py', [('dec', 'handler')]),
    ('dec.py', [('dec', 'handler')]),
    ('dec_override.py', [('dec_override', 'handler_dec')]),
]


@pytest.mark.parametrize('base, expected', test_cases)
def test_find_handlers(base, expected):
    file_name = f'{test_cases_dir}/{base}'
    out = find_handlers(file_name)

    assert sorted(out) == sorted(expected)
