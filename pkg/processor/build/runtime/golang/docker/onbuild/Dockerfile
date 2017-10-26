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

FROM golang:1.8-alpine3.6

# this script builds the processor and shoves the build logs into /processor_build.log
COPY build-processor.sh /usr/local/bin/build-processor

# create nuclio directory under default GOPATH
RUN mkdir -p /go/src/github.com/nuclio
WORKDIR /go/src/github.com/nuclio

# copy nuclio sources from local to the nuclio directory
ONBUILD COPY nuclio /go/src/github.com/nuclio/nuclio
ONBUILD COPY handler /handler
ONBUILD COPY handler-pkg-path.txt /

# We need to "unvendor" the SDK so types in handler and nuclio will be the same.
# TODO: For some reason "mv" creates an empty nuclio-sdk directory
ONBUILD RUN cp -r /go/src/github.com/nuclio/nuclio/vendor/github.com/nuclio/nuclio-sdk \
    /go/src/github.com/nuclio
ONBUILD RUN rm -r /go/src/github.com/nuclio/nuclio/vendor/github.com/nuclio/nuclio-sdk

# run the build script in the nuclio directory
ONBUILD RUN cd nuclio && build-processor
