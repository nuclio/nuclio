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
ARG NUCLIO_DOCKER_REPO=quay.io/nuclio

# Supplies processor
FROM ${NUCLIO_DOCKER_REPO}/processor:${NUCLIO_DOCKER_IMAGE_TAG} as processor

# Supplies wrapper and nuclio-sdk-dotnetcore
FROM mcr.microsoft.com/dotnet/sdk:7.0 as builder

# Update packages
RUN apt-get update && \
    apt-get -s dist-upgrade | \
    grep "^Inst" | \
    awk -F " " {'print $2'} | \
    xargs apt-get install -y --no-install-recommends && \
    rm -rf /var/lib/apt/lists/*

# Copy processor
COPY --from=processor /home/nuclio/bin/processor /home/nuclio/bin/processor

# Fetch Nuclio .NET SDK
RUN git clone \
     --branch master \
     https://github.com/nuclio/nuclio-sdk-dotnetcore.git \
     /home/nuclio/src/nuclio-sdk-dotnetcore

# Copy and build wrapper files
COPY pkg/processor/runtime/dotnetcore /home/nuclio/src/wrapper
RUN dotnet add /home/nuclio/src/wrapper package Microsoft.CSharp && \
    dotnet add /home/nuclio/src/wrapper package System.Dynamic.Runtime && \
    dotnet add /home/nuclio/src/wrapper package System.Runtime.Loader && \
    dotnet add /home/nuclio/src/wrapper package Microsoft.Extensions.DependencyModel && \
    dotnet add /home/nuclio/src/wrapper package Newtonsoft.Json && \
    dotnet add /home/nuclio/src/wrapper reference /home/nuclio/src/nuclio-sdk-dotnetcore/nuclio-sdk-dotnetcore.csproj

# Build the wrapper
RUN cd /home/nuclio/src/wrapper \
    && dotnet restore \
    && dotnet publish -c Release -o /home/nuclio/bin/wrapper

# Copy the proj
COPY pkg/processor/build/runtime/dotnetcore/docker/onbuild/handler.csproj /home/nuclio/src/handler/handler.csproj

# Specify the directory where the handler is kept. By default it is the context dir, but it is overridable
ONBUILD ARG NUCLIO_BUILD_LOCAL_HANDLER_DIR=.

# copy the user code files
ONBUILD COPY ${NUCLIO_BUILD_LOCAL_HANDLER_DIR} /home/nuclio/src/handler

ONBUILD RUN dotnet add /home/nuclio/src/handler package Microsoft.CSharp && \
            dotnet add /home/nuclio/src/handler package System.Dynamic.Runtime && \
            dotnet add /home/nuclio/src/handler package Newtonsoft.Json && \
            dotnet add /home/nuclio/src/handler package Microsoft.Azure.EventHubs -v 2.2.1 && \
            dotnet add /home/nuclio/src/handler reference /home/nuclio/src/nuclio-sdk-dotnetcore/nuclio-sdk-dotnetcore.csproj

ONBUILD RUN cd /home/nuclio/src/handler \
    && dotnet restore \
    && dotnet publish -c Release -o /home/nuclio/bin/handler
