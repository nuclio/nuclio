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
#

FROM golang:1.12

ENV GOOS=linux
ENV GOARCH=amd64

# copy source tree
WORKDIR /go/src/github.com/nuclio/nuclio

ADD cmd     ./cmd
ADD vendor  ./vendor
ADD pkg     ./pkg

# build the dashboard
RUN go get github.com/nuclio/nuclio-sdk-go \
    && go get github.com/nuclio/logger \
    && go get golang.org/x/net/... \
    && go get github.com/v3io/scaler-types \
    && cd ../../v3io/scaler-types \
    && git checkout v1.5.0 \
    && cd /go/src/github.com/nuclio/nuclio
