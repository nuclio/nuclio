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

# BUILD handler.dll

WORKDIR /app
ARG PROJECT_NAME=handler
RUN apt-get update && \
    apt-get install git && \
    git clone -b development https://github.com/torosent/nuclio-sdk-dotnetcore.git /opt/nuclio/nuclio-sdk-dotnetcore
COPY pkg/processor/build/runtime/dotnetcore/docker/onbuild/handler.csproj ${PROJECT_NAME}/
RUN dotnet add ${PROJECT_NAME} package Newtonsoft.Json && \
    dotnet add ${PROJECT_NAME} package Microsoft.Azure.EventHubs -v 2.0.0-preview && \
    dotnet add ${PROJECT_NAME} reference /opt/nuclio/nuclio-sdk-dotnetcore/nuclio-sdk-dotnetcore.csproj
WORKDIR /app/${PROJECT_NAME}

# copy the user code files
ONBUILD COPY handler .

RUN dotnet restore
RUN dotnet publish -c Release -o /opt/nuclio/handler

# BUILD wrapper.dll
WORKDIR /app
ARG PROJECT_NAME=wrapper

# Copy all files and SDK
COPY pkg/processor/runtime/dotnetcore ${PROJECT_NAME}/
RUN dotnet add ${PROJECT_NAME} package MessagePack && \
    dotnet add ${PROJECT_NAME} package MessagePack.ReactiveProperty && \
    dotnet add ${PROJECT_NAME} reference /opt/nuclio/nuclio-sdk-dotnetcore/nuclio-sdk-dotnetcore.csproj
WORKDIR /app/${PROJECT_NAME}

RUN dotnet restore
RUN dotnet publish -c Release -o /opt/nuclio/wrapper

# BUILD runtime image
FROM microsoft/dotnet:2-runtime

COPY --from=processor /go/src/github.com/nuclio/nuclio/processor /usr/local/bin/processor

WORKDIR /opt/nuclio/
COPY --from=build-env /opt/nuclio/ .

# generate a version file
ARG NUCLIO_VERSION_INFO_FILE_CONTENTS
RUN mkdir -p /etc/nuclio && echo ${NUCLIO_VERSION_INFO_FILE_CONTENTS} > /etc/nuclio/version_info.json

