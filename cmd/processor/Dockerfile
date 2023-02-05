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

ARG NUCLIO_LABEL
ARG NUCLIO_DOCKER_REPO

FROM $NUCLIO_DOCKER_REPO/nuclio-builder:$NUCLIO_LABEL as build-processor

ARG NUCLIO_GO_LINK_FLAGS_INJECT_VERSION

# build the processor
RUN go build \
    -a \
    -installsuffix cgo \
    -ldflags="${NUCLIO_GO_LINK_FLAGS_INJECT_VERSION}" \
    -o processor cmd/processor/main.go

FROM scratch

COPY --from=build-processor  /nuclio/processor /home/nuclio/bin/processor
