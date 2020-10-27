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
# Build stage: builds the controller binary
#

ARG NUCLIO_LABEL

FROM nuclio-base:$NUCLIO_LABEL as build-controller

ARG NUCLIO_GO_LINK_FLAGS_INJECT_VERSION

# build the controller
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
    -a \
    -installsuffix cgo \
    -ldflags="${NUCLIO_GO_LINK_FLAGS_INJECT_VERSION}" \
    -o controller cmd/controller/main.go

#
# Output stage: Creates version file, copies binary to an alpine based image
#

FROM alpine:3.11

# install apache2-utils for htpasswd
RUN apk add --no-cache ca-certificates apache2-utils

# copy controller binary from build stage
COPY --from=build-controller /nuclio/controller /usr/local/bin

CMD [ "controller" ]
