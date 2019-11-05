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

FROM golang:1.13 as processor

ENV GOOS=linux
ENV GOARCH=amd64

WORKDIR /go/src/github.com/nuclio/nuclio

# copy go modules first, will help docker caching upon building
COPY go.mod go.sum ./

RUN go mod download

# copy source tree
COPY . .

# build the processor
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags="-s -w" -o /home/nuclio/bin/processor cmd/processor/main.go
