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

ARG NUCLIO_LABEL

FROM nuclio-builder:$NUCLIO_LABEL as builder

FROM gcr.io/iguazio/golang:1.17.5-alpine3.15

ARG GOOS=linux
ARG GOARCH=amd64

ENV GOOS $GOOS
ENV GOARCH $GOARCH

COPY --from=builder $GOPATH/pkg/mod $GOPATH/pkg/mod

RUN apk --update --no-cache add \
    git \
    gcc \
    musl-dev \
    file

WORKDIR /nuclio

COPY . .
