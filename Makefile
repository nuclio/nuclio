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

GO_BUILD=GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags="-s -w"
NUCLIO_CONTROLLER_IMAGE=nuclio/controller
NUCLIO_PLAYGROUND_IMAGE=nuclio/playground
NUCLIO_PROCESSOR_PY_IMAGE=nuclio/processor-py
NUCLIO_PROCESSOR_GOLANG_ONBUILD_IMAGE=nuclio/processor-builder-golang-onbuild
NUCLIO_PROCESSOR_GOLANG_IMAGE=nuclio/processor-builder-golang

all: controller playground nuctl processor-py
	@echo Done.

nuctl: ensure-gopath
	go build -o ${GOPATH}/bin/nuctl cmd/nuctl/main.go

controller:
	${GO_BUILD} -o cmd/controller/_output/controller cmd/controller/main.go
	cd cmd/controller && docker build -t $(NUCLIO_CONTROLLER_IMAGE) .
	rm -rf cmd/controller/_output

# We can't build the processor with CGO_ENABLED=0 since it need to load plugins
processor:
	GOOS=linux GOARCH=amd64 go build -o cmd/processor/_output/processor \
	     -a -installsuffix cgo -ldflags="-s -w" ./cmd/processor

processor-py: processor
	docker build --rm -f pkg/processor/build/runtime/python/docker/processor-py/Dockerfile -t $(NUCLIO_PROCESSOR_PY_IMAGE) .

processor-builder-golang:
	docker build --rm \
	    -t $(NUCLIO_PROCESSOR_GOLANG_IMAGE) \
	    -f pkg/processor/build/runtime/golang/docker/Dockerfile \
	    .

processor-builder-golang-onbuild:
	cd pkg/processor/build/runtime/golang/docker/onbuild && \
	    docker build --rm -t $(NUCLIO_PROCESSOR_GOLANG_ONBUILD_IMAGE) .

playground:
	${GO_BUILD} -o cmd/playground/_output/playground cmd/playground/main.go
	cd cmd/playground && docker build -t $(NUCLIO_PLAYGROUND_IMAGE) .
	rm -rf cmd/playground/_output

.PHONY: install-linters
install-linters:
	go get -u github.com/pavius/impi/cmd/impi
	go get -u gopkg.in/alecthomas/gometalinter.v1
	${GOPATH}/bin/gometalinter.v1 --install

.PHONY: lint
lint:
	@echo Verifying imports...
	@${GOPATH}/bin/impi --local \
	    github.com/nuclio/nuclio/ --scheme stdLocalThirdParty ./cmd/... ./pkg/...

	@echo Linting...
	@${GOPATH}/bin/gometalinter.v1 \
		--disable-all \
		--enable=vet \
		--enable=vetshadow \
		--enable=deadcode \
		--enable=varcheck \
		--enable=staticcheck \
		--enable=gosimple \
		--enable=ineffassign \
		--enable=interfacer \
		--enable=unconvert \
		--enable=goconst \
		--enable=golint \
		--enable=misspell \
		--enable=gofmt \
		--enable=staticcheck \
		--exclude="_test.go" \
		--exclude="should have comment" \
		--exclude="comment on" \
		--exclude="error should be the last" \
		--deadline=300s \
		--concurrency 1 \
		--enable-gc \
		./cmd/... ./pkg/...

	@echo Done.

.PHONY: test
test:
	go test -v ./cmd/... ./pkg/... -p 1

.PHONY: test-python
test-python:
	pytest -v pkg/processor/runtime/python

.PHONY: travis
travis: install-linters lint
	go test -v ./cmd/... ./pkg/... -short

.PHONY: ensure-gopath
check-gopath:
ifndef GOPATH
	$(error GOPATH must be set)
endif
