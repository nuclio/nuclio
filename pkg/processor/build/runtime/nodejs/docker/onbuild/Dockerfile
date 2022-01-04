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

ARG NUCLIO_LABEL=latest
ARG NUCLIO_ARCH=amd64
ARG NUCLIO_DOCKER_REPO=quay.io/nuclio

# Supplies processor
FROM ${NUCLIO_DOCKER_REPO}/processor:${NUCLIO_LABEL}-${NUCLIO_ARCH} as processor

# Doesn't do anything but hold processor binary and all NodeJS code required to run the handler
FROM scratch

COPY --from=processor /home/nuclio/bin/processor /home/nuclio/bin/processor
COPY pkg/processor/runtime/nodejs/js/wrapper.js /home/nuclio/bin/wrapper.js
