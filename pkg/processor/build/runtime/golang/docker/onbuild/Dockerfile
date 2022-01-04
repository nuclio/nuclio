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

# Build the processor
FROM nuclio-base:$NUCLIO_LABEL as build-processor

ARG NUCLIO_GO_LINK_FLAGS_INJECT_VERSION
ARG GOOS=linux
ARG GOARCH=amd64

# Build the processor
RUN GOOS=$GOOS GOARCH=$GOARCH go build \
    -a \
    -installsuffix cgo \
    -ldflags="${NUCLIO_GO_LINK_FLAGS_INJECT_VERSION}" \
    -o /home/nuclio/bin/processor cmd/processor/main.go

# Build the plugin
FROM gcr.io/iguazio/golang:1.17.5

# Store processor binary
COPY --from=build-processor /home/nuclio/bin/processor /home/nuclio/bin/processor

# Store processor go build artifacts
COPY --from=build-processor /nuclio/go.mod /processor_go.mod
COPY --from=build-processor /nuclio/go.sum /processor_go.sum

# Copy the script that builds the plugin
COPY pkg/processor/build/runtime/golang/docker/onbuild/moduler.sh /

# Set handler work dir
WORKDIR /handler

# Specify the directory where the handler is kept. By default it is the context dir, but it is overridable
ONBUILD ARG NUCLIO_BUILD_LOCAL_HANDLER_DIR=.

# Copy handler sources to container, build-handler will move it to the right place
ONBUILD COPY ${NUCLIO_BUILD_LOCAL_HANDLER_DIR} ./

# Specify an onbuild arg to specify offline
ONBUILD ARG NUCLIO_BUILD_OFFLINE

# Run moduler to ensure go modules exists and downloaded
ONBUILD RUN mv /moduler.sh . && chmod +x moduler.sh && sync && ./moduler.sh

# Compile handler as plugin
ONBUILD RUN GOOS=linux GOARCH=amd64 go build -mod=mod -buildmode=plugin -o /home/nuclio/bin/handler.so .
