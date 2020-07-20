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
# Build stage: builds the autoscaler binary
#

ARG NUCLIO_LABEL

FROM nuclio-base-alpine:$NUCLIO_LABEL as build-plugin

ARG NUCLIO_GO_LINK_FLAGS_INJECT_VERSION

RUN GOOS=linux GOARCH=amd64 go build \
    -buildmode=plugin \
    -a \
    -installsuffix cgo \
    -ldflags="${NUCLIO_GO_LINK_FLAGS_INJECT_VERSION}" \
    -o plugin.so pkg/platform/kube/resourcescaler/resourcescaler.go

FROM alpine:3.11

WORKDIR /home/nuclio

COPY --from=build-plugin /nuclio/plugin.so ./plugins/plugin.so
COPY --from=quay.io/v3io/autoscaler:v0.4.0 /home/v3io/bin/autoscaler ./autoscaler

CMD [ "/home/nuclio/autoscaler" ]
