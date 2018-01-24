# Copyright 2017 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#	 http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM openjdk:9-slim

RUN apt-get update
RUN apt-get install -y curl
RUN curl -LO https://services.gradle.org/distributions/gradle-4.4.1-bin.zip
RUN unzip gradle-4.4.1-bin.zip
RUN ln -s /gradle-4.4.1/bin/gradle /usr/local/bin

WORKDIR /nuclio-build
COPY nuclio-sdk-1.0-SNAPSHOT.jar .

ONBUILD COPY . .
ONBUILD RUN gradle shadowJar
