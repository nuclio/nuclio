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

FROM pypy:2-5.9

RUN apt-get install -y gcc make \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* \
    && curl -LO https://dl.google.com/go/go1.10.2.linux-amd64.tar.gz \
    && tar xzf go1.10.2.linux-amd64.tar.gz \
    && rm go1.10.2.linux-amd64.tar.gz \
    && mv go /opt \
    && ln -s /opt/go/bin/go /usr/local/bin \
    && mkdir -p /go \
    && mkdir -p /opt/nuclio/handler

ENV GOROOT=/opt/go GOPATH=/go

COPY pkg/processor/build/runtime/pypy/docker/pypy.pc /usr/share/pkgconfig
COPY pkg/processor/runtime/pypy/nuclio_interface.py /opt/nuclio/handler

RUN go get github.com/v3io/v3io-go-http \
    && go get github.com/nuclio/logger \
    && go get github.com/nuclio/nuclio-sdk-go \
    && go get github.com/nuclio/amqp

# Allow Go runtime to pass object to pypy (via C layer)
ENV GODEBUG="cgocheck=0"

# Make libpypy-c available
RUN ldconfig /usr/local/bin

WORKDIR /go/src/github.com/nuclio/nuclio
COPY . .
RUN go build -tags pypy -a -ldflags="-s -w" -o /processor cmd/processor/main.go

ARG NUCLIO_PYPY_VERSION=2-5.9
ARG NUCLIO_PYPY_OS=jessie

# generate a version file
ARG NUCLIO_VERSION_INFO_FILE_CONTENTS
RUN mkdir -p /etc/nuclio && echo ${NUCLIO_VERSION_INFO_FILE_CONTENTS} > /etc/nuclio/version_info.json
