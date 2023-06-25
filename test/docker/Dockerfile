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

ARG NUCLIO_DOCKER_IMAGE_TAG
ARG NUCLIO_DOCKER_REPO

FROM alpine:latest as builder

# docker
ARG DOCKER_CLI_ARCH=x86_64
ARG DOCKER_CLI_VERSION="23.0.1"
ENV DOCKER_CLI_DOWNLOAD_URL="https://download.docker.com/linux/static/stable/$DOCKER_CLI_ARCH/docker-$DOCKER_CLI_VERSION.tgz"

# kubectl
ARG KUBECTL_CLI_ARCH=amd64
ARG KUBECTL_CLI_VERSION="v1.24.11"
ENV KUBECTL_CLI_DOWNLOAD_URL="https://storage.googleapis.com/kubernetes-release/release/$KUBECTL_CLI_VERSION/bin/linux/$KUBECTL_CLI_ARCH/kubectl"

RUN apk --update --no-cache add curl

# download docker client to /usr/bin/docker
RUN curl $DOCKER_CLI_DOWNLOAD_URL > /tmp/docker.tgz \
     && tar zxf /tmp/docker.tgz --strip=1 -C /usr/bin docker/docker \
     && rm /tmp/docker.tgz

# download docker client to /usr/bin/docker
RUN curl -LO $KUBECTL_CLI_DOWNLOAD_URL \
     && sync \
     && chmod +x ./kubectl \
     && mv ./kubectl /usr/local/bin/kubectl

# ensure both installed correctly
RUN kubectl version --client && docker version | grep "Version: "

FROM $NUCLIO_DOCKER_REPO/nuclio-builder:$NUCLIO_DOCKER_IMAGE_TAG

# copy docker and kubectl cli tools from builder
COPY --from=builder /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=builder /usr/bin/docker /usr/bin/docker
