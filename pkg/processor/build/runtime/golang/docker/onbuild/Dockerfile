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

FROM golang:1.10 as processor

#
# Create processor binary
#

WORKDIR /go/src/github.com/nuclio/nuclio
COPY . .

RUN go get github.com/v3io/v3io-go-http \
    && go get github.com/nuclio/logger \
    && go get github.com/nuclio/nuclio-sdk-go \
    && go get github.com/nuclio/amqp

# Create
RUN mkdir -p /etc/nuclio
RUN mkdir -p /opt/nuclio

# Make the processor binary
RUN mkdir -p /home/nuclio/bin \
    && GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-s -w" -o /home/nuclio/bin/processor cmd/processor/main.go \
    && rm -rf /go/src/github/nuclio/nuclio

FROM golang:1.10

#
# Prepare onbuild
#

# If build passes NUCLIO_ARCH, use it as GOARCH
ARG NUCLIO_ARCH
ENV GOARCH ${NUCLIO_ARCH}

# Copy processor binary
COPY --from=processor /home/nuclio/bin/processor /home/nuclio/bin/processor
COPY --from=processor /go/src/github.com/v3io/v3io-go-http /go/src/github.com/v3io/v3io-go-http
COPY --from=processor /go/src/github.com/nuclio/logger /go/src/github.com/nuclio/logger
COPY --from=processor /go/src/github.com/nuclio/nuclio-sdk-go /go/src/github.com/nuclio/nuclio-sdk-go
COPY --from=processor /go/src/github.com/nuclio/amqp /go/src/github.com/nuclio/amqp

# This script builds the processor
COPY pkg/processor/build/runtime/golang/docker/onbuild/build-handler.sh /

# Specify the directory where the handler is kept. By default it is the context dir, but it is overridable
ONBUILD ARG NUCLIO_BUILD_LOCAL_HANDLER_DIR=.

# Copy handler sources to container, build-handler will move it to the right place
ONBUILD COPY ${NUCLIO_BUILD_LOCAL_HANDLER_DIR} /go/src/github.com/nuclio/handler

# Specify an onbuild arg to specify offline
ONBUILD ARG NUCLIO_BUILD_OFFLINE

# Run build
ONBUILD RUN /build-handler.sh
