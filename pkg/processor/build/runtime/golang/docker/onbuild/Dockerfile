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

FROM golang:1.8-alpine3.6

# this script builds the processor and shoves the build logs into /processor_build.log
COPY build-processor.sh /usr/local/bin/build-processor

# create nuclio directory under default GOPATH
RUN mkdir -p /go/src/github.com/nuclio
WORKDIR /go/src/github.com/nuclio

# copy nuclio sources from local to the nuclio directory
ONBUILD COPY . /go/src/github.com/nuclio

# run the build script in the nuclio directory
ONBUILD RUN cd nuclio && build-processor
