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

# shared
python -m pip install -r py/requirements/common.txt

# dev
python -m pip install -r py/requirements/dev.txt

# determine runtime version and install its packages
if [[ $(python -V 2>&1) =~ 2\.7 ]]; then
    python -m pip install -r py/requirements/python2.txt
else
    python -m pip install -r py/requirements/python3_6.txt
fi

# remove python cached
find ./py \
    -name ".pytest_cache" -type d \
    -o -name "*.pyc" \
    -o -name "__pycache__" -type d \
    | xargs rm -rf

# run tests
python -m pytest -v .
