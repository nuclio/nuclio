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


FROM microsoft/dotnet:2-sdk AS build-env
WORKDIR /app

ARG PROJECT_NAME=wrapper

# Copy all files and SDK
COPY pkg/processor/runtime/dotnetcore ./${PROJECT_NAME}/
COPY pkg/temp /sdk

RUN dotnet add ${PROJECT_NAME} package MessagePack && \
    dotnet add ${PROJECT_NAME} package MessagePack.ReactiveProperty && \
    dotnet add ${PROJECT_NAME} reference /sdk/nuclio-sdk-dotnetcore.csproj

WORKDIR /app/${PROJECT_NAME}

RUN dotnet restore

RUN dotnet publish -c Release -o /opt/nuclio

# Build runtime image
FROM microsoft/dotnet:2-runtime

WORKDIR /opt/nuclio
COPY --from=build-env /opt/nuclio .

# generate a version file
ARG NUCLIO_VERSION_INFO_FILE_CONTENTS
RUN mkdir -p /etc/nuclio && echo ${NUCLIO_VERSION_INFO_FILE_CONTENTS} > /etc/nuclio/version_info.json