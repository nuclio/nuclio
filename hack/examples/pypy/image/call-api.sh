#!/bin/bash

# Copyright 2017 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

case $1 in
    -h | --help ) echo "usage: $(basename $0) PORT"; exit;;
esac

if [ $# -ne 1 ]; then
    1>&2 echo "error: wrong number of arguments"
    exit 1
fi

port=$1

curl \
    --output nuclio-logo-small.png \
    --data-binary @nuclio-logo.png \
    http://localhost:${port}\?ratio\=0.1
