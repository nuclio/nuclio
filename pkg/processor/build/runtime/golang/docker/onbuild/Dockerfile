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

FROM golang:1.9-alpine3.6

RUN apk --update --no-cache add \
    git \
    gcc \
    musl-dev

#
# Prepare onbuild
#

# if build passes NUCLIO_ARCH, use it as GOARCH
ARG NUCLIO_ARCH
ENV GOARCH ${NUCLIO_ARCH}

# This script builds the processor and redirects the build logs into /handler_build.log
COPY pkg/processor/build/runtime/golang/docker/onbuild/build-handler.sh /

# Copy handler sources to container, build-handler will move it to the right place
ONBUILD COPY handler /handler

# Copy handler path to container
ONBUILD COPY handler-pkg-path.txt /

# Run build
ONBUILD RUN /build-handler.sh

#
# Create processor binary
#

WORKDIR /go/src/github.com/nuclio/nuclio
COPY . .

RUN go get -v github.com/nuclio/nuclio-sdk

# create
RUN mkdir -p /etc/nuclio
RUN mkdir -p /opt/nuclio

# make the processor binary
RUN GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-s -w" -o processor cmd/processor/main.go \
    && cp -v /go/src/github.com/nuclio/nuclio/processor /usr/local/bin \
    && rm -r /go/src/github.com/nuclio/nuclio
