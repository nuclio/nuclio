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

all: controller nubuild nuctl processor
	@echo Done.

nubuild: ensure-gopath
	go build -o ${GOPATH}/bin/nubuild cmd/nubuild/main.go

nuctl: ensure-gopath
	go build -o ${GOPATH}/bin/nuctl cmd/nuctl/main.go

controller:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -o cmd/controller/_output/controller -ldflags="-s -w" cmd/controller/main.go
	cd cmd/controller && docker build -t nuclio/controller .
	rm -rf cmd/controller/_output

processor:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" ./cmd/processor

.PHONY: test
#test: vet
test:
	go test -v ./cmd/...
	go test -v ./pkg/...

.PHONY: vet
vet:
	go vet -v ./cmd/...
	go vet -v ./pkg/...

test-py:
	pytest -v pkg/processor/runtime/python/

.PHONY: travis
travis: test

.PHONY: clean
clean:
	rm -f controller
	rm -f nubuild
	rm -f nuctl
	rm -f processor

.PHONY: fresh
fresh: clean all

.PHONY: ensure-gopath
check-gopath:
ifndef GOPATH
    $(error GOPATH must be set)
endif
