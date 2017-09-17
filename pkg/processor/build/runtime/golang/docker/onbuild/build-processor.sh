#!/usr/bin/env bash

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

# compile the processor and redirect all output to /processor_build.log. always return successfully so that
# the image is always created and properly tagged. if processor binary exists, compilation was successful. if it doesn't
# /processor_build.log should explain why
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go get -a -installsuffix cgo github.com/nuclio/nuclio/cmd/processor > /processor_build.log 2>&1 || true
