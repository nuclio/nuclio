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

FROM golang:1.10

# copy source tree
WORKDIR /go/src/github.com/nuclio/nuclio
COPY . .

# build the processor
RUN go get github.com/v3io/v3io-go-http \
    && go get github.com/nuclio/logger \
    && go get github.com/nuclio/nuclio-sdk-go \
    && go get github.com/nuclio/amqp \
    && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags="-s -w" -o /home/nuclio/bin/processor cmd/processor/main.go
