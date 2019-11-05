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

set -e

cd /go/src/$1

# moving the go.mod & go.sum to the right place if needed
if [ ! -f "./go.mod" ]
then
    mv /go/go.mod ./go.mod
    mv /go/go.sum ./go.sum
else

    # we dont need it, we HAVE remove it
    rm /go/go.mod /go/go.sum
fi

# since we are using Nuclio projects go.mod & sum, we need to replace the module package accordingly to the new handler
go mod edit --module $1

# if specified to build offline, skip go get
if [ "${NUCLIO_BUILD_OFFLINE}" != "true" ]; then

    # omit unneeded packages
    go mod tidy
    go mod download
fi

# if go deps succeeded, build plugin
if [ $? -eq 0 ]; then
    go build -buildmode=plugin -o /home/nuclio/bin/handler.so
fi
