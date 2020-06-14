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

ARG NUCLIO_LABEL

FROM nuclio-base:$NUCLIO_LABEL as build-binary

ARG NUCLIO_GO_LINK_FLAGS_INJECT_VERSION

# build the playground
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags="${NUCLIO_GO_LINK_FLAGS_INJECT_VERSION}" -o playground cmd/playground/main.go

#
# Output stage: Creates version file, copies binary to an alpine based image
#

FROM alpine:3.11

ARG DOCKER_CLI_VERSION="18.09.6"

ENV DOWNLOAD_URL="https://download.docker.com/linux/static/stable/x86_64/docker-$DOCKER_CLI_VERSION.tgz"

RUN apk --update --no-cache add ca-certificates git curl \
    && mkdir -p /tmp/download \
    && curl -L $DOWNLOAD_URL | tar -xz -C /tmp/download \
    && mv /tmp/download/docker/docker /usr/local/bin/ \
    && rm -rf /tmp/download \
    && apk del curl

# copy playground binary from build stage
COPY --from=build-playground /nuclio/playground /usr/local/bin
COPY cmd/playground/static /etc/nuclio/playground/assets

RUN mkdir -p /etc/nuclio/playground/sources

CMD ["playground", "--assets-dir", "/etc/nuclio/playground/assets", "--sources-dir", "/etc/nuclio/playground/sources", "--docker-key-dir", "/etc/nuclio/playground/registry-credentials", "--listen-addr", ":8070"]
