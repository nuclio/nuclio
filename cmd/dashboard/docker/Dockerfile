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
# Build assets stage: builds the dashboard assets (js, html, css, etc)
#
ARG NUCLIO_LABEL

FROM node:10.21.0 as build-static

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
FROM nuclio-base:$NUCLIO_LABEL as build-binary

ARG NUCLIO_GO_LINK_FLAGS_INJECT_VERSION

# build the dashboard
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
    -a \
    -installsuffix cgo \
    -ldflags="${NUCLIO_GO_LINK_FLAGS_INJECT_VERSION}" \
    -o dashboard cmd/dashboard/main.go

#
# Output stage: Creates version file, copies binary and assets to an alpine image
#

FROM nginx:stable-alpine

ARG DOCKER_CLI_VERSION="19.03.12"

ENV DOWNLOAD_URL="https://download.docker.com/linux/static/stable/x86_64/docker-$DOCKER_CLI_VERSION.tgz"

RUN apk --update --no-cache add ca-certificates git curl parallel \
    && mkdir -p /tmp/download \
    && curl -L $DOWNLOAD_URL | tar -xz -C /tmp/download \
    && mv /tmp/download/docker/docker /usr/local/bin/ \
    && rm -rf /tmp/download \
    && apk del curl

# copy runners
COPY cmd/dashboard/docker/runners /runners
COPY cmd/dashboard/docker/runner.sh /runner.sh

# make them executable
RUN chmod +x /runners/*.sh /runner.sh

# copy nginx config
COPY cmd/dashboard/docker/nginx.conf /etc/nginx/conf.d/default.conf

# copy dashboard static from build assets stage. /etc/nuclio/dashboard/static will contain index.html and assets
COPY --from=build-static /home/nuclio/dashboard/src/dist /etc/nginx/static/

# copy dashboard binary from build binary stage
COPY --from=build-binary /nuclio/dashboard /usr/local/bin

CMD ["sh",  "-c", "./runner.sh"]
