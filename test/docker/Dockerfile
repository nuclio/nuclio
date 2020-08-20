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

FROM nuclio-base:$NUCLIO_LABEL

ARG DOCKER_CLI_VERSION="18.09.6"
ARG KUBECTL_CLI_VERSION="v1.17.9"

ENV DOCKER_DOWNLOAD_URL="https://download.docker.com/linux/static/stable/x86_64/docker-$DOCKER_CLI_VERSION.tgz"
ENV KUBECTL_DOWNLOAD_URL="https://storage.googleapis.com/kubernetes-release/release/$KUBECTL_CLI_VERSION/bin/linux/amd64/kubectl"

# download docker client to /usr/bin/docker
RUN curl $DOCKER_DOWNLOAD_URL > /tmp/docker.tgz \
     && tar zxf /tmp/docker.tgz --strip=1 -C /usr/bin docker/docker \
     && rm /tmp/docker.tgz

# download docker client to /usr/bin/docker
RUN curl -LO $KUBECTL_DOWNLOAD_URL \
     && sync \
     && chmod +x ./kubectl \
     && mv ./kubectl /usr/local/bin/kubectl

# ensure both installed correctly
RUN kubectl version --client
