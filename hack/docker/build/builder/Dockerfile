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

ARG NUCLIO_BASE_IMAGE_NAME
ARG NUCLIO_BASE_IMAGE_TAG

FROM ${NUCLIO_BASE_IMAGE_NAME}:${NUCLIO_BASE_IMAGE_TAG}

ARG GOOS=linux
ARG GOARCH=amd64
ARG NUCLIO_GO_PROXY=${NUCLIO_GO_PROXY}

ENV GOOS $GOOS
ENV GOARCH $GOARCH
ENV GOPROXY $NUCLIO_GO_PROXY

WORKDIR /nuclio

COPY go.mod go.sum ./

RUN go mod download
