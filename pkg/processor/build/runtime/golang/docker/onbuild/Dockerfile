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

RUN apk update
RUN apk add \
    make \
    gcc \
    git \
    musl-dev

RUN go get -v github.com/nuclio/nuclio-sdk

# This script builds the processor and redirects the build logs into /handler_build.log
COPY build-handler.sh /

# Copy handler sources to container, build-handler will move it to the right place
ONBUILD COPY handler /handler

# Copy handler path to container
ONBUILD COPY handler-pkg-path.txt /
# Run build
ONBUILD RUN /build-handler.sh
