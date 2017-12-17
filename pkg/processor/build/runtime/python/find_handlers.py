#!/usr/bin/env python
"""Find handlers"""

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

from contextlib import contextmanager
from os.path import dirname, basename
import sys


is_py3 = sys.version_info[:2] >= (3, 0)

if is_py3:
    from inspect import signature

    def has_handler_signature(fn):
        num_free = 0
        has_varargs = False

        for param in signature(fn).parameters.values():
            if param.kind == param.VAR_POSITIONAL:
                has_varargs = True
                continue
            if param.default == param.empty:
                num_free += 1

        if num_free == 2:
            return True
        return num_free < 2 and has_varargs

else:
    from inspect import getargspec

    def has_handler_signature(fn):
        spec = getargspec(fn)

        num_free = len(spec.args or []) - len(spec.defaults or [])
        if num_free == 2:
            return True

        return num_free < 2 and spec.varargs


@contextmanager
def add_path(dir_name):
    """Templrarly prepend dir_name to sys.path"""
    sys.path.insert(0, dir_name)
    try:
        yield dir_name
    finally:
        sys.path.pop(0)


def load_module(mod_name, file_name):
    """Load module from file"""
    with add_path(dirname(file_name)):
        module = __import__(mod_name)

    return module


def find_handlers(py_file):
    """Find handlers in Python file.

    A handler is callable that has `nuclio_handler` attributes. If no such
    callable is found, all callables with 2 arguments are returned.
    """
    handlers = []
    possible_handlers = []

    # '/path/to/module.py' -> 'module'
    mod_name = basename(py_file)[:-3]
    module = load_module(mod_name, py_file)
    for name, obj in vars(module).items():
        if not callable(obj):
            continue

        if getattr(obj, 'nuclio_handler', None):
            handlers.append((mod_name, name))
            continue

        if has_handler_signature(obj):
            possible_handlers.append((mod_name, name))
            continue

    return handlers or possible_handlers


if __name__ == '__main__':
    from argparse import ArgumentParser, FileType
    import json

    parser = ArgumentParser()
    parser.add_argument('path', help='path to discover', type=FileType())
    args = parser.parse_args()

    handlers = find_handlers(args.path.name)
    out = [{'module': mod, 'handler': hdlr} for mod, hdlr in handlers]
    json.dump(out, sys.stdout)
