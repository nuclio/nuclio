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

GO_BUILD=GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags="-s -w"

all: controller playground nuctl
	@echo Done.

nuctl: ensure-gopath
	go build -o ${GOPATH}/bin/nuctl cmd/nuctl/main.go

controller:
	${GO_BUILD} -o cmd/controller/_output/controller cmd/controller/main.go
	cd cmd/controller && docker build -t nuclio/controller .
	rm -rf cmd/controller/_output

processor:
	${GO_BUILD} -o cmd/processor/_output/processor cmd/processor/main.go

processor-py: processor
	docker build --rm -f pkg/processor/build/runtime/python/docker/processor-py/Dockerfile -t nuclio/processor-py .

processor-builder-golang-onbuild:
	cd pkg/processor/build/runtime/golang/docker/onbuild && docker build --rm -t nuclio/processor-builder-golang-onbuild .

playground:
	${GO_BUILD} -o cmd/playground/_output/playground cmd/playground/main.go
	cd cmd/playground && docker build -t nuclio/playground .
	rm -rf cmd/playground/_output

.PHONY: vet
vet:
	go vet -v ./cmd/...
	go vet -v ./pkg/...

.PHONY: test
test:
	go test -v ./cmd/...
	go test -v ./pkg/...

.PHONY: travis
travis: vet test

.PHONY: ensure-gopath
check-gopath:
ifndef GOPATH
    $(error GOPATH must be set)
endif
