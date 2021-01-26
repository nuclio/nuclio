#!/bin/bash

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

# exit on failure
set -o errexit

# show command before execute
set -o xtrace

# remove python cached
find ./py \
    -name ".pytest_cache" -type d \
    -o -name "*.pyc" \
    -o -name "__pycache__" -type d \
    -print0 \
    | xargs rm -rf

# run tests
python -m pytest -v .
