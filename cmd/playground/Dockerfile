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

#
# Build stage: builds the playground binary
#

FROM golang:1.9.2 as build

# copy source tree
WORKDIR /go/src/github.com/nuclio/nuclio
COPY . .

# build the playground
RUN go get github.com/v3io/v3io-go-http \
    && go get github.com/nuclio/nuclio-sdk \
    && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags="-s -w" -o playground cmd/playground/main.go

#
# Output stage: Creates version file, copies binary to an alpine based image
#

FROM alpine:3.6

ARG DOCKER_CLI_VERSION="17.09.0-ce"

ENV DOWNLOAD_URL="https://download.docker.com/linux/static/stable/x86_64/docker-$DOCKER_CLI_VERSION.tgz"

RUN apk --update --no-cache add ca-certificates git curl \
    && mkdir -p /tmp/download \
    && curl -L $DOWNLOAD_URL | tar -xz -C /tmp/download \
    && mv /tmp/download/docker/docker /usr/local/bin/ \
    && rm -rf /tmp/download \
    && apk del curl

# copy playground binary from build stage
COPY --from=build /go/src/github.com/nuclio/nuclio/playground /usr/local/bin
COPY cmd/playground/static /etc/nuclio/playground/assets

RUN mkdir -p /etc/nuclio/playground/sources

# generate a version file
ARG NUCLIO_VERSION_INFO_FILE_CONTENTS
RUN mkdir -p /etc/nuclio && echo ${NUCLIO_VERSION_INFO_FILE_CONTENTS} > /etc/nuclio/version_info.json

CMD ["playground", "--assets-dir", "/etc/nuclio/playground/assets", "--sources-dir", "/etc/nuclio/playground/sources", "--docker-key-dir", "/etc/nuclio/playground/docker-keys", "--listen-addr", ":8070"]
