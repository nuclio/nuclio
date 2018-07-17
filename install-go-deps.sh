#!/bin/sh

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
# Install Go dependencies for nuclio (ones which we can't vendor)
#

case $1 in
    -h | --help ) echo "usage: $(basename $0)"; exit;;
esac

set -e
set -x

go get github.com/v3io/v3io-go-http
go get github.com/nuclio/logger
go get github.com/nuclio/nuclio-sdk-go
go get github.com/nuclio/amqp

# TODO: Remove once dealer-interface is merged in nuclio-sdk-go
cd $(go env GOPATH)/src/github.com/nuclio/nuclio-sdk-go
git remote add tebeka https://github.com/tebeka/nuclio-sdk-go
git fetch tebeka
git checkout dealer-interface
