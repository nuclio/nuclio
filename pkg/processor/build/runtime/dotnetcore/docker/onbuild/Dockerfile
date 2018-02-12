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

ARG PROJECT_NAME=userfunction

COPY pkg/processor/build/runtime/dotnetcore/docker/onbuild/userfunction.csproj ./${PROJECT_NAME}/
COPY pkg/temp /sdk

RUN dotnet add ${PROJECT_NAME} package Newtonsoft.Json && \
    dotnet add ${PROJECT_NAME} package Microsoft.Azure.EventHubs -v 2.0.0-preview && \
    dotnet add ${PROJECT_NAME} reference /sdk/nuclio-sdk-dotnetcore.csproj

WORKDIR /app/${PROJECT_NAME}

# copy the user code files
ONBUILD COPY handler .

# restore as distinct layers
RUN dotnet restore
RUN dotnet publish -c Release -o /opt/nuclio/handler

# Build runtime image
FROM microsoft/dotnet:2-runtime
WORKDIR /opt/nuclio/handler
COPY --from=build-env /opt/nuclio/handler .

RUN mkdir -p /etc/nuclio

