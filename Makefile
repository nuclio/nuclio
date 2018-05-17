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
OS_NAME = $(shell uname)

# get default os / arch from go env
NUCLIO_DEFAULT_OS := $(shell go env GOOS)
NUCLIO_DEFAULT_ARCH := $(shell go env GOARCH)

ifeq ($(OS_NAME), Linux)
	NUCLIO_DEFAULT_TEST_HOST := $(shell docker network inspect bridge | grep "Gateway" | grep -o '"[^"]*"$$')
	# On EC2 we don't have gateway, use default
	ifeq ($(NUCLIO_DEFAULT_TEST_HOST),)
	    NUCLIO_DEFAULT_TEST_HOST := "172.17.0.1"
	endif
else
	NUCLIO_DEFAULT_TEST_HOST := "docker.for.mac.host.internal"
endif

NUCLIO_OS := $(if $(NUCLIO_OS),$(NUCLIO_OS),$(NUCLIO_DEFAULT_OS))
NUCLIO_ARCH := $(if $(NUCLIO_ARCH),$(NUCLIO_ARCH),$(NUCLIO_DEFAULT_ARCH))
NUCLIO_LABEL := $(if $(NUCLIO_LABEL),$(NUCLIO_LABEL),latest)
NUCLIO_TEST_HOST := $(if $(NUCLIO_TEST_HOST),$(NUCLIO_TEST_HOST),$(NUCLIO_DEFAULT_TEST_HOST))
NUCLIO_VERSION_GIT_COMMIT = $(shell git rev-parse HEAD)

NUCLIO_VERSION_INFO = {\"git_commit\": \"$(NUCLIO_VERSION_GIT_COMMIT)\",  \
\"label\": \"$(NUCLIO_LABEL)\",  \
\"os\": \"$(NUCLIO_OS)\",  \
\"arch\": \"$(NUCLIO_ARCH)\"}

# Dockerized tests variables - not available for changes
NUCLIO_DOCKER_TEST_DOCKERFILE_PATH := test/docker/Dockerfile
NUCLIO_DOCKER_TEST_TAG := nuclio/tester

# Add labels to docker images
NUCLIO_DOCKER_LABELS = --label nuclio.version_info="$(NUCLIO_VERSION_INFO)"

NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_LABEL)-$(NUCLIO_ARCH)

# Link flags
GO_LINK_FLAGS ?= -s -w
GO_LINK_FLAGS_INJECT_VERSION := $(GO_LINK_FLAGS) -X github.com/nuclio/nuclio/pkg/version.gitCommit=$(NUCLIO_VERSION_GIT_COMMIT) \
	-X github.com/nuclio/nuclio/pkg/version.label=$(NUCLIO_LABEL) \
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
	--volume $(shell pwd):$(GO_BUILD_TOOL_WORKDIR) \
	--volume $(shell pwd)/../nuclio-sdk-go:$(GO_BUILD_TOOL_WORKDIR)/../nuclio-sdk-go \
	--volume $(shell pwd)/../logger:$(GO_BUILD_TOOL_WORKDIR)/../logger \
	--volume $(GOPATH)/bin:/go/bin \
	--workdir $(GO_BUILD_TOOL_WORKDIR) \
	--env GOOS=$(NUCLIO_OS) \
	--env GOARCH=$(NUCLIO_ARCH) \
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
    dashboard \
    processor \
    handler-builder-golang-onbuild \
    handler-builder-java-onbuild \
    handler-builder-python-onbuild \
    handler-builder-dotnetcore-onbuild \
    handler-builder-nodejs-onbuild

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

NUCTL_BIN_NAME = nuctl-$(NUCLIO_LABEL)-$(NUCLIO_OS)-$(NUCLIO_ARCH)
NUCTL_TARGET = $(GOPATH)/bin/nuctl

nuctl: ensure-gopath
	$(GO_BUILD_TOOL) -o /go/bin/$(NUCTL_BIN_NAME) cmd/nuctl/main.go
	@rm -f $(NUCTL_TARGET)
	@ln -sF $(GOPATH)/bin/$(NUCTL_BIN_NAME) $(NUCTL_TARGET)

processor: ensure-gopath
	docker build --file cmd/processor/Dockerfile --tag nuclio/processor:$(NUCLIO_DOCKER_IMAGE_TAG) .

#
# Dockerized services
#

# Controller
NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME=nuclio/controller:$(NUCLIO_DOCKER_IMAGE_TAG)

controller: ensure-gopath
	docker build $(NUCLIO_BUILD_ARGS_VERSION_INFO_FILE) \
		--file cmd/controller/Dockerfile \
		--tag $(NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME) \
		$(NUCLIO_DOCKER_LABELS) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME)

# Playground
NUCLIO_DOCKER_PLAYGROUND_IMAGE_NAME=nuclio/playground:$(NUCLIO_DOCKER_IMAGE_TAG)

playground: ensure-gopath
	docker build $(NUCLIO_BUILD_ARGS_VERSION_INFO_FILE) \
		--file cmd/playground/Dockerfile \
		--tag $(NUCLIO_DOCKER_PLAYGROUND_IMAGE_NAME) \
		$(NUCLIO_DOCKER_LABELS) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_PLAYGROUND_IMAGE_NAME)

# Dashboard
NUCLIO_DOCKER_DASHBOARD_IMAGE_NAME=nuclio/dashboard:$(NUCLIO_DOCKER_IMAGE_TAG)

dashboard: ensure-gopath
	docker build $(NUCLIO_BUILD_ARGS_VERSION_INFO_FILE) \
		--file cmd/dashboard/docker/Dockerfile \
		--tag $(NUCLIO_DOCKER_DASHBOARD_IMAGE_NAME) \
		$(NUCLIO_DOCKER_LABELS) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_DASHBOARD_IMAGE_NAME)

#
# Onbuild images
#

# Python
NUCLIO_DOCKER_HANDLER_BUILDER_PYTHON_ONBUILD_IMAGE_NAME=\
nuclio/handler-builder-python-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)

handler-builder-python-onbuild:
	docker build --build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) --build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file pkg/processor/build/runtime/python/docker/onbuild/Dockerfile \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_PYTHON_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_PYTHON_ONBUILD_IMAGE_NAME)

# Go
NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME=\
nuclio/handler-builder-golang-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)

NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME=\
$(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME)-alpine

handler-builder-golang-onbuild:
	docker build --build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) --build-arg NUCLIO_LABEL=$(NUCLIO_LABEL)  \
		--file pkg/processor/build/runtime/golang/docker/onbuild/Dockerfile \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME) .

	docker build --build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) --build-arg NUCLIO_LABEL=$(NUCLIO_LABEL)  \
		--file pkg/processor/build/runtime/golang/docker/onbuild/Dockerfile.alpine \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME) \
    $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME)

# Pypy
NUCLIO_DOCKER_PROCESSOR_PYPY_JESSIE_IMAGE_NAME=nuclio/processor-pypy2-5.9-jessie:$(NUCLIO_DOCKER_IMAGE_TAG)

processor-pypy:
	docker build $(NUCLIO_BUILD_ARGS) \
		--file pkg/processor/build/runtime/pypy/docker/Dockerfile.processor-pypy \
		--build-arg NUCLIO_PYPY_VERSION=2-5.9 \
		--build-arg NUCLIO_PYPY_OS=jessie \
		--tag $(NUCLIO_DOCKER_PROCESSOR_PYPY_JESSIE_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_PROCESSOR_PYPY_JESSIE_IMAGE_NAME)

NUCLIO_DOCKER_HANDLER_BUILDER_PYPY_ONBUILD_IMAGE_NAME=nuclio/handler-pypy2-5.9-jessie:$(NUCLIO_DOCKER_IMAGE_TAG)

handler-pypy:
	docker build \
		--file pkg/processor/build/runtime/pypy/docker/Dockerfile.handler-pypy \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_DOCKER_IMAGE_TAG) \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_PYPY_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_PYPY_ONBUILD_IMAGE_NAME)

# NodeJS
NUCLIO_DOCKER_HANDLER_BUILDER_NODEJS_ONBUILD_IMAGE_NAME=\
nuclio/handler-builder-nodejs-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)

handler-builder-nodejs-onbuild:
	docker build --build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) --build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file pkg/processor/build/runtime/nodejs/docker/onbuild/Dockerfile \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_NODEJS_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_NODEJS_ONBUILD_IMAGE_NAME)

# dotnet core
NUCLIO_DOCKER_HANDLER_BUILDER_DOTNETCORE_ONBUILD_IMAGE_NAME=nuclio/handler-builder-dotnetcore-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)
NUCLIO_ONBUILD_DOTNETCORE_DOCKERFILE_PATH = pkg/processor/build/runtime/dotnetcore/docker/onbuild/Dockerfile

handler-builder-dotnetcore-onbuild: processor
	docker build --build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) --build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		-f $(NUCLIO_ONBUILD_DOTNETCORE_DOCKERFILE_PATH) \
		-t $(NUCLIO_DOCKER_HANDLER_BUILDER_DOTNETCORE_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_DOTNETCORE_ONBUILD_IMAGE_NAME)

# java
NUCLIO_DOCKER_HANDLER_BUILDER_JAVA_ONBUILD_IMAGE_NAME=\
nuclio/handler-builder-java-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)

handler-builder-java-onbuild:
	docker build --build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) --build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file pkg/processor/build/runtime/java/docker/onbuild/Dockerfile \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_JAVA_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_JAVA_ONBUILD_IMAGE_NAME)

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
	$(GOPATH)/bin/impi \
        --local github.com/nuclio/nuclio/ \
        --scheme stdLocalThirdParty \
        --skip pkg/platform/kube/apis \
        --skip pkg/platform/kube/client \
        ./cmd/... ./pkg/...

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
		--enable=unconvert \
		--enable=varcheck \
		--enable=vet \
		--enable=vetshadow \
		--enable=errcheck \
		--exclude="_test.go" \
		--exclude="comment on" \
		--exclude="error should be the last" \
		--exclude="should have comment" \
		--skip=pkg/platform/kube/apis \
		--skip=pkg/platform/kube/client \
		./cmd/... ./pkg/...

	@echo Done.

.PHONY: test-undockerized
test-undockerized: ensure-gopath
	go test -v ./cmd/... ./pkg/... -p 1

.PHONY: test
test: ensure-gopath
	docker build --file $(NUCLIO_DOCKER_TEST_DOCKERFILE_PATH) \
	--tag $(NUCLIO_DOCKER_TEST_TAG) .

	docker run --rm --volume /var/run/docker.sock:/var/run/docker.sock \
	--volume $(shell pwd):$(GO_BUILD_TOOL_WORKDIR) \
	--volume /tmp:/tmp \
	--workdir /go/src/github.com/nuclio/nuclio \
	--env NUCLIO_TEST_HOST=$(NUCLIO_TEST_HOST) \
	$(NUCLIO_DOCKER_TEST_TAG) \
	/bin/bash -c "make test-undockerized"

.PHONY: test-python
test-python:
	docker build -f pkg/processor/runtime/python/test/Dockerfile.py3-test .
	docker build -f pkg/processor/runtime/python/test/Dockerfile.py2-test .

.PHONY: test-short
test-short: ensure-gopath
	go test -v ./cmd/... ./pkg/... -short

.PHONY: ensure-gopath
ensure-gopath:
ifndef GOPATH
	$(error GOPATH must be set)
endif
