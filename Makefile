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
GOPATH ?= $(shell go env GOPATH)

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
GO_LINK_FLAGS ?= -s -w
GO_LINK_FLAGS_INJECT_VERSION := $(GO_LINK_FLAGS) -X github.com/nuclio/nuclio/pkg/version.gitCommit=$(NUCLIO_VERSION_GIT_COMMIT) \
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

DOCKER_IMAGES_RULES = \
    controller \
    playground \
    processor-py \
    handler-builder-golang-onbuild \
    processor-shell \
    processor-pypy \
    handler-pypy \
    handler-nodejs

docker-images: ensure-gopath $(DOCKER_IMAGES_RULES)
	@echo Done.

tools: ensure-gopath nuctl
	@echo Done.

push-docker-images:
	for image in $(IMAGES_TO_PUSH); do \
		docker push $$image ; \
	done
	@echo Done.

print-docker-images:
	for image in $(IMAGES_TO_PUSH); do \
		echo $$image ; \
	done

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
	docker build -f cmd/processor/Dockerfile -t nuclio/processor .

#
# Dockerized services
#

# Controller
NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME=nuclio/controller:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)

controller: ensure-gopath
	docker build $(NUCLIO_BUILD_ARGS_VERSION_INFO_FILE) \
		-f cmd/controller/Dockerfile \
		-t $(NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME) \
		$(NUCLIO_DOCKER_LABELS) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME)

# Playground
NUCLIO_DOCKER_PLAYGROUND_IMAGE_NAME=nuclio/playground:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)

playground: ensure-gopath
	docker build $(NUCLIO_BUILD_ARGS_VERSION_INFO_FILE) \
		-f cmd/playground/Dockerfile \
		-t $(NUCLIO_DOCKER_PLAYGROUND_IMAGE_NAME) \
		$(NUCLIO_DOCKER_LABELS) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_PLAYGROUND_IMAGE_NAME)

# Python
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

IMAGES_TO_PUSH += \
	$(NUCLIO_DOCKER_PROCESSOR_PY2_ALPINE_IMAGE_NAME) \
	$(NUCLIO_DOCKER_PROCESSOR_PY2_JESSIE_IMAGE_NAME) \
	$(NUCLIO_DOCKER_PROCESSOR_PY3_ALPINE_IMAGE_NAME) \
	$(NUCLIO_DOCKER_PROCESSOR_PY3_JESSIE_IMAGE_NAME)

# Go
NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME=nuclio/handler-builder-golang-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)

handler-builder-golang-onbuild: ensure-gopath
	docker build --build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) \
		-f pkg/processor/build/runtime/golang/docker/onbuild/Dockerfile \
		-t $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME)

# Pypy
NUCLIO_DOCKER_PROCESSOR_PYPY_JESSIE_IMAGE_NAME=nuclio/processor-pypy2-5.9-jessie:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)

processor-pypy:
	docker build $(NUCLIO_BUILD_ARGS) \
		-f pkg/processor/build/runtime/pypy/docker/Dockerfile.processor-pypy \
		--build-arg NUCLIO_PYPY_VERSION=2-5.9 \
		--build-arg NUCLIO_PYPY_OS=jessie \
		-t $(NUCLIO_DOCKER_PROCESSOR_PYPY_JESSIE_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_PROCESSOR_PYPY_JESSIE_IMAGE_NAME)

NUCLIO_DOCKER_HANDLER_BUILDER_PYPY_ONBUILD_IMAGE_NAME=nuclio/handler-pypy2-5.9-jessie:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)

handler-pypy:
	docker build \
		-f pkg/processor/build/runtime/pypy/docker/Dockerfile.handler-pypy \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH=$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH) \
		-t $(NUCLIO_DOCKER_HANDLER_BUILDER_PYPY_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_PYPY_ONBUILD_IMAGE_NAME)

# Shell
NUCLIO_PROCESSOR_SHELL_DOCKERFILE_PATH = pkg/processor/build/runtime/shell/docker/processor-shell/Dockerfile
NUCLIO_DOCKER_PROCESSOR_SHELL_ALPINE_IMAGE_NAME=nuclio/processor-shell-alpine:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)

processor-shell: processor
	# build shell/alpine
	docker build $(NUCLIO_BUILD_ARGS_VERSION_INFO_FILE) \
	-f $(NUCLIO_PROCESSOR_SHELL_DOCKERFILE_PATH) \
	-t $(NUCLIO_DOCKER_PROCESSOR_SHELL_ALPINE_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_PROCESSOR_SHELL_ALPINE_IMAGE_NAME)

# nodejs
NUCLIO_HANDLER_NODEJS_DOCKERFILE_PATH = pkg/processor/build/runtime/nodejs/docker/Dockerfile.handler-nodejs
NUCLIO_DOCKER_HANDLER_NODEJS_ALPINE_IMAGE_NAME=nuclio/handler-nodejs:$(NUCLIO_DOCKER_IMAGE_TAG_WITH_ARCH)

handler-nodejs: processor
	docker build $(NUCLIO_BUILD_ARGS_VERSION_INFO_FILE) \
	-f $(NUCLIO_HANDLER_NODEJS_DOCKERFILE_PATH) \
	-t $(NUCLIO_DOCKER_HANDLER_NODEJS_ALPINE_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_NODEJS_ALPINE_IMAGE_NAME)

#
# Testing
#
.PHONY: lint
lint: ensure-gopath
	@echo Installing linters...
	go get -u github.com/pavius/impi/cmd/impi
	go get -u gopkg.in/alecthomas/gometalinter.v2
	@$(GOPATH)/bin/gometalinter.v2 --install

	@echo Verifying imports...
	$(GOPATH)/bin/impi --local github.com/nuclio/nuclio/ --scheme stdLocalThirdParty ./cmd/... ./pkg/...
	@echo Linting...
	@$(GOPATH)/bin/gometalinter.v2 \
		--deadline=300s \
		--disable-all \
		--enable-gc \
		--enable=deadcode \
		--enable=goconst \
		--enable=gofmt \
		--enable=golint \
		--enable=gosimple \
		--enable=ineffassign \
		--enable=interfacer \
		--enable=misspell \
		--enable=staticcheck \
		--enable=staticcheck \
		--enable=unconvert \
		--enable=varcheck \
		--enable=vet \
		--enable=vetshadow \
		--exclude="_test.go" \
		--exclude="comment on" \
		--exclude="error should be the last" \
		--exclude="should have comment" \
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
