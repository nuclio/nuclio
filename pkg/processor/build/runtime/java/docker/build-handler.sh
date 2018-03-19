#!/usr/bin/env sh

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

# Compile the handler and redirect all output to /handler_build.log.  Always
# return successfully so that the image is always created and properly tagged.
# If handler DLL exists (/handler.so), compilation was successful. if it
# doesn't /handler_build.log should explain why

# Re run script with output redirected to /nuclio-build/build.log and always exit
# successfully

if [ -z "${RUN_REDIRECT}" ]; then
    # Make sure the directory is there
    mkdir -p /nuclio-build

    RUN_REDIRECT=1 $0 $@ > /nuclio-build/build.log 2>&1
    exit 0
fi

set -e

gradle nuclioJar
