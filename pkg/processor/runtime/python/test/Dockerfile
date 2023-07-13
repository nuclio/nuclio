# Copyright 2023 The Nuclio Authors.
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

ARG PYTHON_IMAGE_TAG=3.6
FROM gcr.io/iguazio/python:$PYTHON_IMAGE_TAG

WORKDIR /nuclio

# copy common/dev requirements file
COPY pkg/processor/runtime/python/py/requirements/dev.txt py/requirements/dev.txt
COPY pkg/processor/runtime/python/py/requirements/common.txt py/requirements/common.txt

# install common / dev
RUN python -m pip install \
    --requirement py/requirements/common.txt \
    --requirement py/requirements/dev.txt

# copy files
COPY pkg/processor/runtime/python .

# determine runtime version and install its packages (if exists)
ARG PYTHON_IMAGE_TAG=3.6
RUN RUNTIME_REQUIREMENTS_SPECIFIC_FILE="py/requirements/python$(echo "$PYTHON_IMAGE_TAG" | tr '.' '_').txt" \
    && (test -f ${RUNTIME_REQUIREMENTS_SPECIFIC_FILE} \
        && python -m pip install -r ${RUNTIME_REQUIREMENTS_SPECIFIC_FILE}) \
        || true

ARG CACHEBUST=1
RUN ./test/test.sh
