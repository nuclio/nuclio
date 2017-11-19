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

GO_VERSION := $(shell go version | cut -d " " -f 3)

# get default os / arch from go env
NUCLIO_DEFAULT_OS := $(shell go env GOOS)
NUCLIO_DEFAULT_ARCH := $(shell go env GOARCH)

NUCLIO_OS := $(if $(NUCLIO_OS),$(NUCLIO_OS),$(NUCLIO_DEFAULT_OS))
NUCLIO_ARCH := $(if $(NUCLIO_ARCH),$(NUCLIO_ARCH),$(NUCLIO_DEFAULT_ARCH))
NUCLIO_TAG := $(if $(NUCLIO_TAG),$(NUCLIO_TAG),latest)
NUCLIO_VERSION_GIT_COMMIT = $(shell git rev-parse HEAD)

NUCLIO_VERSION_INFO = {\"git_commit\": \"$(NUCLIO_VERSION_GIT_COMMIT)\",  \
\"label\": \"$(NUCLIO_TAG)\",  \
\"os\": \"$(NUCLIO_OS)\",  \
\"arch\": \"$(NUCLIO_ARCH)\"}

# Add labels to docker images
NUCLIO_DOCKER_LABELS = --label nuclio.version_info="$(NUCLIO_VERSION_INFO)"

NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_TAG)
NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH=$(NUCLIO_TAG)-$(NUCLIO_ARCH)

# Link flags
GO_LINK_FLAGS := -s -w
GO_LINK_FLAGS_INJECT_VERSION := -s -w -X github.com/nuclio/nuclio/pkg/version.gitCommit=$(NUCLIO_VERSION_GIT_COMMIT) \
	-X github.com/nuclio/nuclio/pkg/version.label=$(NUCLIO_TAG) \
	-X github.com/nuclio/nuclio/pkg/version.os=$(NUCLIO_OS) \
	-X github.com/nuclio/nuclio/pkg/version.arch=$(NUCLIO_ARCH)

# inject version info as file
NUCLIO_BUILD_ARGS_VERSION_INFO_FILE = --build-arg NUCLIO_VERSION_INFO_FILE_CONTENTS="$(NUCLIO_VERSION_INFO)"


#
# Build helpers
#

# tools get built with the specified OS/arch and inject version
GO_BUILD_TOOL_WORKDIR = /go/src/github.com/nuclio/nuclio
GO_BUILD_TOOL = docker run \
	-v $(shell pwd):$(GO_BUILD_TOOL_WORKDIR) \
	-v $(shell pwd)/../nuclio-sdk:$(GO_BUILD_TOOL_WORKDIR)/../nuclio-sdk \
	-v $(GOPATH)/bin:/go/bin \
	-w $(GO_BUILD_TOOL_WORKDIR) \
	-e GOOS=$(NUCLIO_OS) \
	-e GOARCH=$(NUCLIO_ARCH) \
	golang:1.9.2 \
	go build -a \
	-installsuffix cgo \
	-ldflags="$(GO_LINK_FLAGS_INJECT_VERSION)"

#
# Rules
#

build: docker-images tools
	@echo Done.

docker-images: ensure-gopath controller playground processor-py handler-builder-golang-onbuild
	@echo Done.

tools: ensure-gopath nuctl
	@echo Done.

push-docker-images: controller-push playground-push processor-py-push handler-builder-golang-onbuild-push
	@echo Done.

#
# Tools
#

NUCTL_BIN_NAME = nuctl-$(NUCLIO_TAG)-$(NUCLIO_OS)-$(NUCLIO_ARCH)
NUCTL_TARGET = $(GOPATH)/bin/nuctl

nuctl: ensure-gopath
	$(GO_BUILD_TOOL) -o /go/bin/$(NUCTL_BIN_NAME) cmd/nuctl/main.go
	@rm -f $(NUCTL_TARGET)
	@ln -sF $(GOPATH)/bin/$(NUCTL_BIN_NAME) $(NUCTL_TARGET)

processor: ensure-gopath
	$(eval NUCLIO_OS := linux)
	docker build -f cmd/processor/Dockerfile -t nuclio/processor .

#
# Dockerized services
#

NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME=nuclio/controller:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)

controller: ensure-gopath
	$(eval NUCLIO_OS := linux)
	docker build $(NUCLIO_BUILD_ARGS_VERSION_INFO_FILE) \
		-f cmd/controller/Dockerfile \
		-t $(NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME) \
		$(NUCLIO_DOCKER_LABELS) .

controller-push:
	docker push $(NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME)

NUCLIO_DOCKER_PLAYGROUND_IMAGE_NAME=nuclio/playground:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)

playground: ensure-gopath
	$(eval NUCLIO_OS := linux)
	docker build $(NUCLIO_BUILD_ARGS_VERSION_INFO_FILE) \
		-f cmd/playground/Dockerfile \
		-t $(NUCLIO_DOCKER_PLAYGROUND_IMAGE_NAME) \
		$(NUCLIO_DOCKER_LABELS) .

playground-push:
	docker push $(NUCLIO_DOCKER_PLAYGROUND_IMAGE_NAME)

#
# Base images
#

NUCLIO_PROCESSOR_PY_DOCKERFILE_PATH = pkg/processor/build/runtime/python/docker/processor-py/Dockerfile
NUCLIO_DOCKER_PROCESSOR_PY2_ALPINE_IMAGE_NAME=nuclio/processor-py2.7-alpine:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)
NUCLIO_DOCKER_PROCESSOR_PY3_ALPINE_IMAGE_NAME=nuclio/processor-py3.6-alpine:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)
NUCLIO_DOCKER_PROCESSOR_PY2_JESSIE_IMAGE_NAME=nuclio/processor-py2.7-jessie:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)
NUCLIO_DOCKER_PROCESSOR_PY3_JESSIE_IMAGE_NAME=nuclio/processor-py3.6-jessie:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)

processor-py: processor

	# build python 2.7/alpine
	docker build $(NUCLIO_BUILD_ARGS_VERSION_INFO_FILE) \
		-f ${NUCLIO_PROCESSOR_PY_DOCKERFILE_PATH} \
		--build-arg NUCLIO_PYTHON_VERSION=2.7 \
		--build-arg NUCLIO_PYTHON_OS=alpine3.6 \
		-t $(NUCLIO_DOCKER_PROCESSOR_PY2_ALPINE_IMAGE_NAME) .

	# build python 3/alpine
	docker build $(NUCLIO_BUILD_ARGS_VERSION_INFO_FILE) \
		-f ${NUCLIO_PROCESSOR_PY_DOCKERFILE_PATH} \
		--build-arg NUCLIO_PYTHON_VERSION=3.6 \
		--build-arg NUCLIO_PYTHON_OS=alpine3.6 \
		-t $(NUCLIO_DOCKER_PROCESSOR_PY3_ALPINE_IMAGE_NAME) .

	# build python 2/jesse
	docker build $(NUCLIO_BUILD_ARGS_VERSION_INFO_FILE) \
		-f ${NUCLIO_PROCESSOR_PY_DOCKERFILE_PATH} \
		--build-arg NUCLIO_PYTHON_VERSION=2.7 \
		--build-arg NUCLIO_PYTHON_OS=slim-jessie \
		-t $(NUCLIO_DOCKER_PROCESSOR_PY2_JESSIE_IMAGE_NAME) .

	# build python 3/jesse
	docker build $(NUCLIO_BUILD_ARGS_VERSION_INFO_FILE) \
		-f ${NUCLIO_PROCESSOR_PY_DOCKERFILE_PATH} \
		--build-arg NUCLIO_PYTHON_VERSION=3.6 \
		--build-arg NUCLIO_PYTHON_OS=slim-jessie \
		-t $(NUCLIO_DOCKER_PROCESSOR_PY3_JESSIE_IMAGE_NAME) .

processor-py-push:
	docker push $(NUCLIO_DOCKER_PROCESSOR_PY2_ALPINE_IMAGE_NAME)
	docker push $(NUCLIO_DOCKER_PROCESSOR_PY3_ALPINE_IMAGE_NAME)
	docker push $(NUCLIO_DOCKER_PROCESSOR_PY2_JESSIE_IMAGE_NAME)
	docker push $(NUCLIO_DOCKER_PROCESSOR_PY3_JESSIE_IMAGE_NAME)

NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME=nuclio/handler-builder-golang-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)

handler-builder-golang-onbuild: ensure-gopath
	docker build --build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) \
		-f pkg/processor/build/runtime/golang/docker/onbuild/Dockerfile \
		-t $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME) .

handler-builder-golang-onbuild-push:
	docker push $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME)

#
# Testing
#

.PHONY: lint
lint: ensure-gopath
	@echo Verifying imports...
	@go get -u github.com/pavius/impi/cmd/impi
	@$(GOPATH)/bin/impi --local github.com/nuclio/nuclio/ --scheme stdLocalThirdParty ./cmd/... ./pkg/...

	@echo Linting...
	@go get -u gopkg.in/alecthomas/gometalinter.v1
	@$(GOPATH)/bin/gometalinter.v1 --install
	@$(GOPATH)/bin/gometalinter.v1 \
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
test: ensure-gopath
	go test -v ./cmd/... ./pkg/... -p 1

.PHONY: test-python
test-python: ensure-gopath
	pytest -v pkg/processor/runtime/python

.PHONY: test-short
test-short: ensure-gopath
	go test -v ./cmd/... ./pkg/... -short

.PHONY: ensure-gopath
ensure-gopath:
ifndef GOPATH
	$(error GOPATH must be set)
endif
