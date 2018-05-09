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

ARG NUCLIO_TAG=latest
ARG NUCLIO_ARCH=amd64

# Supplies processor
FROM nuclio/processor:${NUCLIO_TAG}-${NUCLIO_ARCH} as processor

FROM openjdk:9-slim as user-handler-builder

RUN apt-get update \
    && apt-get install -y wget \
    && wget -q https://services.gradle.org/distributions/gradle-4.5.1-bin.zip \
    && unzip gradle-4.5.1-bin.zip \
    && rm gradle-4.5.1-bin.zip \
    && ln -s /gradle-4.5.1/bin/gradle /usr/local/bin \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Copy processor
COPY --from=processor /home/nuclio/bin/processor /home/nuclio/bin/processor

#
# Build user handler jar
#

# Create user handler directory
RUN mkdir -p /home/nuclio/src/userHandler

# Copy the gradle build script and user sources to WORKDIR
ONBUILD COPY handler /home/nuclio/src/userHandler/

# Run the handle builder to create /home/nuclio/src/userHandler/build/libs/user-handler.jar. Then copy the output.jar
# to /home/nuclio/src/wrapper/user-handler.jar in preparation of building wrapper
ONBUILD RUN cd /home/nuclio/src/userHandler \
    && gradle tasks \
    && gradle userHandler

#
# Build wrapper
#

# Create wrapper directory
RUN mkdir -p /home/nuclio/src/wrapper

# Copy user-handler generated earlier to wrapper
ONBUILD RUN mv /home/nuclio/src/userHandler/build/libs/user-handler.jar /home/nuclio/src/wrapper/user-handler.jar

# The directory will hold wrapper source, SDK jar, user-handler.jar and wrapper gradle build script
COPY pkg/processor/runtime/java/build.gradle \
    pkg/processor/runtime/java/nuclio-sdk-1.0-SNAPSHOT.jar \
    pkg/processor/runtime/java/src \
    /home/nuclio/src/wrapper/

# Download dependencies
RUN cd /home/nuclio/src/wrapper \
    && gradle compileJava

# Build nuclio-java-wrapper.jar
ONBUILD RUN cd /home/nuclio/src/wrapper \
    && gradle wrapperJar
