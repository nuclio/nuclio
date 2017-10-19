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
