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

# Supplies python 3.6 & common wheels
FROM gcr.io/iguazio/python:3.6 as python36-builder

COPY pkg/processor/runtime/python/py/requirements /requirements

# Python 3.6 wheels
RUN pip download \
        --dest /whl \
        --exists-action i \
        --requirement /requirements/python3_6.txt

# Supplies python 3.7 & common wheels
FROM gcr.io/iguazio/python:3.7 as python37-builder

COPY pkg/processor/runtime/python/py/requirements /requirements

# Python 3.7 wheels
RUN pip download \
        --dest /whl \
        --exists-action i \
        --requirement /requirements/python3_7.txt

# Supplies python 3.8 & common wheels
FROM gcr.io/iguazio/python:3.8 as python38-builder

COPY pkg/processor/runtime/python/py/requirements /requirements

# Python 3.8 wheels
RUN pip download \
        --dest /whl \
        --exists-action i \
        --requirement /requirements/python3_8.txt

# Supplies python 3.9 & common wheels
FROM python:3.9 as python39-builder

COPY pkg/processor/runtime/python/py/requirements /requirements

# Python 3.9 wheels
RUN pip download \
        --dest /whl \
        --exists-action i \
        --requirement /requirements/python3_9.txt

# Supplies processor
FROM ${NUCLIO_DOCKER_REPO}/processor:${NUCLIO_LABEL}-${NUCLIO_ARCH} as processor

# Doesn't do anything but hold processor binary and all Python code required to run the handler
FROM scratch

COPY pkg/processor/runtime/python/py /home/nuclio/bin/py

# Copy processor
COPY --from=processor /home/nuclio/bin/processor /home/nuclio/bin/processor

# Copy wheels
COPY --from=python36-builder /whl /home/nuclio/bin/py-whl
COPY --from=python37-builder /whl /home/nuclio/bin/py3.7-whl
COPY --from=python38-builder /whl /home/nuclio/bin/py3.8-whl
COPY --from=python39-builder /whl /home/nuclio/bin/py3.9-whl
