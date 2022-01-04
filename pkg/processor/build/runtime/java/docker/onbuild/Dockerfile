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

ARG NUCLIO_LABEL=latest
ARG NUCLIO_ARCH=amd64
ARG NUCLIO_DOCKER_REPO=quay.io/nuclio

# Supplies processor
FROM ${NUCLIO_DOCKER_REPO}/processor:${NUCLIO_LABEL}-${NUCLIO_ARCH} as processor

FROM gcr.io/iguazio/openjdk:9-slim as user-handler-builder

RUN apt-get update -qqy \
    && apt-get install -o APT::Immediate-Configure=false -qqy curl \
    && curl -LO https://services.gradle.org/distributions/gradle-4.5.1-bin.zip \
    && unzip gradle-4.5.1-bin.zip \
    && rm gradle-4.5.1-bin.zip \
    && ln -s /gradle-4.5.1/bin/gradle /usr/local/bin \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir /home/gradle

WORKDIR /home/gradle

# Copy processor
COPY --from=processor /home/nuclio/bin/processor /home/gradle/bin/processor

#
# Build user handler jar
#

RUN mkdir -p /home/gradle/src/userHandler

# Copy the user handler builder script
COPY pkg/processor/build/runtime/java/docker/onbuild/build-user-handler.sh /home/gradle/src/userHandler/build-user-handler.sh
RUN chmod +x /home/gradle/src/userHandler/build-user-handler.sh

# Download the nuclio SDK Jar
RUN curl -LO https://repo1.maven.org/fromsearch?filepath=io/nuclio/nuclio-sdk-java/1.0.0/nuclio-sdk-java-1.0.0.jar

# Copy the downloaded SDK Jar to the userHandler folder
RUN cp nuclio-sdk-java-1.0.0.jar /home/gradle/src/userHandler/nuclio-sdk-java-1.0.0.jar

# Copy the gradle build script and user sources to /home/gradle/src/userHandler
ONBUILD COPY handler/build.gradle /home/gradle/src/userHandler

# Specify the directory where the handler is kept. By default it is the context dir, but it is overridable
ONBUILD ARG NUCLIO_BUILD_LOCAL_HANDLER_DIR=.

# Copy the entire code to /home/gradle/src/userHandler, where gradle expects it to reside. Note that
# this will also copy build.gradle... but we'll ignore it
ONBUILD COPY ${NUCLIO_BUILD_LOCAL_HANDLER_DIR} /home/gradle/src/userHandler

# Run the handle builder to create /home/gradle/src/userHandler/build/libs/user-handler.jar.
ONBUILD RUN cd /home/gradle/src/userHandler \
    && ./build-user-handler.sh

#
# Build wrapper
#

# Create wrapper directory
RUN mkdir -p /home/gradle/src/wrapper

# Copy user-handler generated earlier to wrapper
ONBUILD RUN mv /home/gradle/src/userHandler/build/libs/user-handler.jar /home/gradle/src/wrapper/user-handler.jar

# The directory will hold wrapper source, SDK jar, user-handler.jar and wrapper gradle build script
COPY pkg/processor/runtime/java/build.gradle \
    pkg/processor/runtime/java/docker \
    pkg/processor/build/runtime/java/docker/onbuild/build-wrapped-handler.sh \
    /home/gradle/src/wrapper/

# Copy also the downloaded nuclio SDK Jar to the wrapper folder
RUN cp nuclio-sdk-java-1.0.0.jar /home/gradle/src/wrapper/nuclio-sdk-java-1.0.0.jar

# Download dependencies
RUN cd /home/gradle/src/wrapper \
    && gradle --build-cache compileJava

# Build nuclio-java-wrapper.jar
ONBUILD RUN /home/gradle/src/wrapper/build-wrapped-handler.sh
