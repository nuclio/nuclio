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
# Build stage: builds the dlx binary
#

ARG NUCLIO_LABEL
ARG ALPINE_IMAGE=gcr.io/iguazio/alpine:3.15

FROM nuclio-base:$NUCLIO_LABEL as build-dlx

ARG NUCLIO_GO_LINK_FLAGS_INJECT_VERSION

ARG GOOS=linux
ARG GOARCH=amd64

# build dlx
RUN GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build \
    -a \
    -installsuffix cgo \
    -ldflags="${NUCLIO_GO_LINK_FLAGS_INJECT_VERSION}" \
    -o dlx cmd/dlx/main.go

FROM $ALPINE_IMAGE

RUN apk add --no-cache ca-certificates

# copy dlx binary from build stage
COPY --from=build-dlx /nuclio/dlx /usr/local/bin

CMD [ "dlx" ]
