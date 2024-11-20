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

#
# Build assets stage: builds the dashboard assets (js, html, css, etc)
#
ARG NGINX_IMAGE=gcr.io/iguazio/nginx:1.25-alpine
ARG NUCLIO_DOCKER_IMAGE_TAG
ARG UHTTPC_IMAGE=gcr.io/iguazio/uhttpc:0.0.1
ARG UHTTPC_ARCH=amd64
ARG NUCLIO_DOCKER_ALPINE_IMAGE
ARG NUCLIO_DOCKER_REPO
ARG NODE_IMAGE_NAME

FROM $NODE_IMAGE_NAME as build-static

# copy source tree
COPY ./pkg/dashboard/ui /home/nuclio/dashboard/src

# install gulp and bower, cd into the source dir and build to create /home/nuclio/dashboard/src/dist
# which contains all the necessary files (index.html and assets/ dir)
RUN npm install -g gulp \
    && cd /home/nuclio/dashboard/src \
    && rm -rf ./dist ./node_modules ./resources/*/node_modules \
    && npm install --production \
    && gulp build --production

#
# Build binary stage: builds the dashboard binary
#
FROM $NUCLIO_DOCKER_REPO/nuclio-builder:$NUCLIO_DOCKER_IMAGE_TAG as build-binary

ARG NUCLIO_GO_LINK_FLAGS_INJECT_VERSION

RUN go build \
    -ldflags="${NUCLIO_GO_LINK_FLAGS_INJECT_VERSION}" \
    -o dashboard cmd/dashboard/main.go


FROM $UHTTPC_IMAGE-$UHTTPC_ARCH AS uhttpc

#
# Output stage: Creates version file, copies binary and assets to an alpine image
#

FROM $NUCLIO_DOCKER_ALPINE_IMAGE as downloaddocker

# docker client
ARG DOCKER_CLI_ARCH=x86_64
ARG DOCKER_CLI_VERSION="23.0.1"
ENV DOCKER_CLI_DOWNLOAD_URL="https://download.docker.com/linux/static/stable/$DOCKER_CLI_ARCH/docker-$DOCKER_CLI_VERSION.tgz"

RUN apk --update --no-cache add curl \
    && mkdir -p /tmp/download \
    && curl -L $DOCKER_CLI_DOWNLOAD_URL | tar -xz -C /tmp/download


FROM $NGINX_IMAGE

# fetch nuclio functions templates during build time to ease installation on offline environments
ARG NUCLIO_TEMPLATES_ZIP_GITHUB_URL="https://github.com/nuclio/nuclio-templates/archive/master.zip"

# copy dashboard static from build assets stage. /etc/nuclio/dashboard/static will contain index.html and assets
COPY --from=build-static /home/nuclio/dashboard/src/dist /etc/nginx/static/

# copy dashboard binary from build binary stage
COPY --from=build-binary /nuclio/dashboard /usr/local/bin

# copy a lightweight http client
COPY --from=uhttpc /home/nuclio/bin/uhttpc /usr/local/bin/uhttpc

# copy docker client
COPY --from=downloaddocker /tmp/download/docker/docker /usr/local/bin

# copy runners
COPY cmd/dashboard/docker/runners /runners
COPY cmd/dashboard/docker/runner.sh /runner.sh

# copy nginx config
COPY cmd/dashboard/docker/default.conf /etc/nginx/conf.d/default.conf
COPY cmd/dashboard/docker/nginx.conf /etc/nginx/nginx.conf

RUN apk --update --no-cache add \
        parallel \
        ca-certificates \
        git \
    && apk upgrade --no-cache \
    && wget -O /tmp/templates.zip $NUCLIO_TEMPLATES_ZIP_GITHUB_URL

HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://127.0.0.1:8082/ready || exit 1

CMD ["sh",  "-c", "./runner.sh"]
