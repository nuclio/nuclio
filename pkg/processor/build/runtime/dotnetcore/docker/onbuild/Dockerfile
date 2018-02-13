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


# make the processor binary
FROM nuclio/processor AS processor

#*********************************************

FROM microsoft/dotnet:2-sdk AS build-env

# Get .NET core SDK 
WORKDIR /app
RUN apt-get update && \
    apt-get install git && \
    git clone --depth=1 -b development https://github.com/torosent/nuclio-sdk-dotnetcore.git /opt/nuclio/nuclio-sdk-dotnetcore && \
    rm -Rf /opt/nuclio/nuclio-sdk-dotnetcore/.git/

ARG PROJECT_NAME=handler
COPY pkg/processor/build/runtime/dotnetcore/docker/onbuild/handler.csproj ${PROJECT_NAME}/

# BUILD wrapper.dll
ARG PROJECT_NAME=wrapper
# Copy and build wrapper files
COPY pkg/processor/runtime/dotnetcore ${PROJECT_NAME}/
RUN dotnet add ${PROJECT_NAME} package MessagePack -v 1.7.3.4 && \
    dotnet add ${PROJECT_NAME} package MessagePack.ReactiveProperty -v 1.7.3.4 && \
    dotnet add ${PROJECT_NAME} reference /opt/nuclio/nuclio-sdk-dotnetcore/nuclio-sdk-dotnetcore.csproj
WORKDIR /app/${PROJECT_NAME}

RUN dotnet restore
RUN dotnet publish -c Release -o /opt/nuclio/wrapper

# Copy processor
COPY --from=processor /go/src/github.com/nuclio/nuclio/processor /usr/local/bin/processor

# BUILD handler.dll ONBUILD

ONBUILD WORKDIR /app
ONBUILD ARG PROJECT_NAME=handler
ONBUILD RUN dotnet add ${PROJECT_NAME} package Newtonsoft.Json -v 10.0.3 && \
            dotnet add ${PROJECT_NAME} package Microsoft.Azure.EventHubs -v 2.0.0-preview && \
            dotnet add ${PROJECT_NAME} reference /opt/nuclio/nuclio-sdk-dotnetcore/nuclio-sdk-dotnetcore.csproj

ONBUILD WORKDIR /app/${PROJECT_NAME}

# copy the user code files
ONBUILD COPY /function .
ONBUILD RUN dotnet restore
ONBUILD RUN dotnet publish -c Release -o /opt/nuclio/handler

WORKDIR /opt/nuclio

# generate a version file
#ARG NUCLIO_VERSION_INFO_FILE_CONTENTS
#RUN mkdir -p /etc/nuclio && echo ${NUCLIO_VERSION_INFO_FILE_CONTENTS} > /etc/nuclio/version_info.json

