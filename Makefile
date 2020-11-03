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
KUBECONFIG := $(if $(KUBECONFIG),$(KUBECONFIG),$(HOME)/.kube/config)

# upstream repo
NUCLIO_DOCKER_REPO ?= quay.io/nuclio

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

NUCLIO_VERSION_INFO = {\"git_commit\": \"$(NUCLIO_VERSION_GIT_COMMIT)\", \"label\": \"$(NUCLIO_LABEL)\"}

# Dockerized tests variables - not available for changes
NUCLIO_DOCKER_TEST_DOCKERFILE_PATH := test/docker/Dockerfile
NUCLIO_DOCKER_TEST_TAG := nuclio/tester

# Add labels to docker images
NUCLIO_DOCKER_LABELS = --label nuclio.version_info="$(NUCLIO_VERSION_INFO)"

NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_LABEL)-$(NUCLIO_ARCH)

# Link flags
GO_LINK_FLAGS ?= -s -w
GO_LINK_FLAGS_INJECT_VERSION := $(GO_LINK_FLAGS) \
	-X github.com/v3io/version-go.gitCommit=$(NUCLIO_VERSION_GIT_COMMIT) \
	-X github.com/v3io/version-go.label=$(NUCLIO_LABEL)

# Docker client version to be used
DOCKER_CLI_VERSION := 19.03.12

# Nuclio test timeout
NUCLIO_GO_TEST_TIMEOUT ?= "30m"

#
#  Must be first target
#
all:
	$(error "Please pick a target (run 'make targets' to view targets)")


#
# Version resources
#

helm-publish:
	$(eval HELM_PUBLISH_COMMIT_MESSAGE := "Releasing chart $(shell helm inspect chart hack/k8s/helm/nuclio | yq r - version)")
	@echo Fetching branch
	@rm -rf /tmp/nuclio-helm
	@git clone -b gh-pages --single-branch git@github.com:nuclio/nuclio.git /tmp/nuclio-helm
	@echo Creating package and updating index
	@helm package -d /tmp/nuclio-helm/charts hack/k8s/helm/nuclio
	@cd /tmp/nuclio-helm/charts && helm repo index --merge index.yaml --url https://nuclio.github.io/nuclio/charts/ .
	@echo Publishing
	@cd /tmp/nuclio-helm/charts && git add --all && git commit -m $(HELM_PUBLISH_COMMIT_MESSAGE) && git push origin
	@echo Done

#
# Build helpers
#

# tools get built with the specified OS/arch and inject version
GO_BUILD_TOOL_WORKDIR = /nuclio
GO_BUILD_NUCTL = docker run \
	--volume $(GOPATH)/bin:/go/bin \
	--env GOOS=$(NUCLIO_OS) \
	--env GOARCH=$(NUCLIO_ARCH) \
	nuclio-base:$(NUCLIO_LABEL) \
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
	dashboard \
	processor \
	autoscaler \
	dlx \
	handler-builder-golang-onbuild \
	handler-builder-java-onbuild \
	handler-builder-ruby-onbuild \
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

NUCLIO_NUCTL_CREATE_SYMLINK := $(if $(NUCLIO_NUCTL_CREATE_SYMLINK),$(NUCLIO_NUCTL_CREATE_SYMLINK),true)
NUCTL_BIN_NAME = nuctl-$(NUCLIO_LABEL)-$(NUCLIO_OS)-$(NUCLIO_ARCH)
NUCTL_TARGET = $(GOPATH)/bin/nuctl

nuctl: ensure-gopath build-base
	$(GO_BUILD_NUCTL) -o /go/bin/$(NUCTL_BIN_NAME) cmd/nuctl/main.go
	@rm -f $(NUCTL_TARGET)
ifeq ($(NUCLIO_NUCTL_CREATE_SYMLINK), true)
	@ln -sF $(GOPATH)/bin/$(NUCTL_BIN_NAME) $(NUCTL_TARGET)
endif

processor: ensure-gopath build-base
	docker build \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file cmd/processor/Dockerfile \
		--tag $(NUCLIO_DOCKER_REPO)/processor:$(NUCLIO_DOCKER_IMAGE_TAG) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_REPO)/processor:$(NUCLIO_DOCKER_IMAGE_TAG)

#
# Dockerized services
#

# Controller
NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME=$(NUCLIO_DOCKER_REPO)/controller:$(NUCLIO_DOCKER_IMAGE_TAG)

controller: ensure-gopath build-base
	docker build \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file cmd/controller/Dockerfile \
		--tag $(NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME) \
		$(NUCLIO_DOCKER_LABELS) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME)

# Dashboard
NUCLIO_DOCKER_DASHBOARD_IMAGE_NAME=$(NUCLIO_DOCKER_REPO)/dashboard:$(NUCLIO_DOCKER_IMAGE_TAG)

dashboard: ensure-gopath build-base
	docker build \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--build-arg DOCKER_CLI_VERSION=$(DOCKER_CLI_VERSION) \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file cmd/dashboard/docker/Dockerfile \
		--tag $(NUCLIO_DOCKER_DASHBOARD_IMAGE_NAME) \
		$(NUCLIO_DOCKER_LABELS) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_DASHBOARD_IMAGE_NAME)

# Scaler
NUCLIO_DOCKER_SCALER_IMAGE_NAME=$(NUCLIO_DOCKER_REPO)/autoscaler:$(NUCLIO_DOCKER_IMAGE_TAG)

autoscaler: ensure-gopath build-base
	docker build \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file cmd/autoscaler/Dockerfile \
		--tag $(NUCLIO_DOCKER_SCALER_IMAGE_NAME) \
		$(NUCLIO_DOCKER_LABELS) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_SCALER_IMAGE_NAME)

# Dlx
NUCLIO_DOCKER_DLX_IMAGE_NAME=$(NUCLIO_DOCKER_REPO)/dlx:$(NUCLIO_DOCKER_IMAGE_TAG)

dlx: ensure-gopath build-base
	docker build \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file cmd/dlx/Dockerfile \
		--tag $(NUCLIO_DOCKER_DLX_IMAGE_NAME) \
		$(NUCLIO_DOCKER_LABELS) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_DLX_IMAGE_NAME)

#
# Onbuild images
#

# Python
NUCLIO_DOCKER_HANDLER_BUILDER_PYTHON_ONBUILD_IMAGE_NAME=\
$(NUCLIO_DOCKER_REPO)/handler-builder-python-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)

PIP_REQUIRE_VIRTUALENV=false

handler-builder-python-onbuild:
	docker build \
		--build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file pkg/processor/build/runtime/python/docker/onbuild/Dockerfile \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_PYTHON_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_PYTHON_ONBUILD_IMAGE_NAME)

# Go
NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME=\
 $(NUCLIO_DOCKER_REPO)/handler-builder-golang-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)

NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME=\
 $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME)-alpine

handler-builder-golang-onbuild-alpine: build-base
	docker build \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file pkg/processor/build/runtime/golang/docker/onbuild/Dockerfile.alpine \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME) .

handler-builder-golang-onbuild: build-base handler-builder-golang-onbuild-alpine
	docker build \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file pkg/processor/build/runtime/golang/docker/onbuild/Dockerfile \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME) \
	$(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME)

# Pypy
NUCLIO_DOCKER_PROCESSOR_PYPY_JESSIE_IMAGE_NAME=$(NUCLIO_DOCKER_REPO)/processor-pypy2-5.9-jessie:$(NUCLIO_DOCKER_IMAGE_TAG)

processor-pypy:
	docker build \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--file pkg/processor/build/runtime/pypy/docker/Dockerfile.processor-pypy \
		--build-arg NUCLIO_PYPY_VERSION=2-5.9 \
		--build-arg NUCLIO_PYPY_OS=jessie \
		--tag $(NUCLIO_DOCKER_PROCESSOR_PYPY_JESSIE_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_PROCESSOR_PYPY_JESSIE_IMAGE_NAME)

NUCLIO_DOCKER_HANDLER_BUILDER_PYPY_ONBUILD_IMAGE_NAME=$(NUCLIO_DOCKER_REPO)/handler-pypy2-5.9-jessie:$(NUCLIO_DOCKER_IMAGE_TAG)

handler-pypy:
	docker build \
		--file pkg/processor/build/runtime/pypy/docker/Dockerfile.handler-pypy \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_DOCKER_IMAGE_TAG) \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_PYPY_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_PYPY_ONBUILD_IMAGE_NAME)

# NodeJS
NUCLIO_DOCKER_HANDLER_BUILDER_NODEJS_ONBUILD_IMAGE_NAME=\
$(NUCLIO_DOCKER_REPO)/handler-builder-nodejs-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)

handler-builder-nodejs-onbuild:
	docker build \
		--build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file pkg/processor/build/runtime/nodejs/docker/onbuild/Dockerfile \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_NODEJS_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_NODEJS_ONBUILD_IMAGE_NAME)

# Ruby
NUCLIO_DOCKER_HANDLER_BUILDER_RUBY_ONBUILD_IMAGE_NAME=\
$(NUCLIO_DOCKER_REPO)/handler-builder-ruby-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)

handler-builder-ruby-onbuild:
	docker build \
		--build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file pkg/processor/build/runtime/ruby/docker/onbuild/Dockerfile \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_RUBY_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_RUBY_ONBUILD_IMAGE_NAME)


# dotnet core
NUCLIO_DOCKER_HANDLER_BUILDER_DOTNETCORE_ONBUILD_IMAGE_NAME=$(NUCLIO_DOCKER_REPO)/handler-builder-dotnetcore-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)
NUCLIO_ONBUILD_DOTNETCORE_DOCKERFILE_PATH = pkg/processor/build/runtime/dotnetcore/docker/onbuild/Dockerfile

handler-builder-dotnetcore-onbuild: processor
	docker build \
		--build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		-f $(NUCLIO_ONBUILD_DOTNETCORE_DOCKERFILE_PATH) \
		-t $(NUCLIO_DOCKER_HANDLER_BUILDER_DOTNETCORE_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_DOTNETCORE_ONBUILD_IMAGE_NAME)

# java
NUCLIO_DOCKER_HANDLER_BUILDER_JAVA_ONBUILD_IMAGE_NAME=\
$(NUCLIO_DOCKER_REPO)/handler-builder-java-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)

handler-builder-java-onbuild:
	docker build \
		--build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file pkg/processor/build/runtime/java/docker/onbuild/Dockerfile \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_JAVA_ONBUILD_IMAGE_NAME) .

IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_JAVA_ONBUILD_IMAGE_NAME)

.PHONY: modules
modules: ensure-gopath
	@echo Getting go modules
	@go mod download

#
# Testing
#
.PHONY: lint
lint: modules
	@echo Installing linters...
	@test -e $(GOPATH)/bin/impi || \
		mkdir -p $(GOPATH)/bin && \
		curl -s https://api.github.com/repos/pavius/impi/releases/latest \
			| grep -i "browser_download_url.*impi.*$(OS_NAME)" \
			| cut -d : -f 2,3 \
			| tr -d \" \
			| wget -O $(GOPATH)/bin/impi -qi -
	@test -e $(GOPATH)/bin/golangci-lint || \
	  	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin v1.27.0

	@echo Verifying imports...
	chmod +x $(GOPATH)/bin/impi && $(GOPATH)/bin/impi \
		--local github.com/nuclio/nuclio/ \
		--scheme stdLocalThirdParty \
		--skip pkg/platform/kube/apis \
		--skip pkg/platform/kube/client \
		./cmd/... ./pkg/... ./hack/...

	@echo Linting...
	$(GOPATH)/bin/golangci-lint run -v
	@echo Done.

.PHONY: test-undockerized
test-undockerized: ensure-gopath
	go test -v -p 1 --timeout $(NUCLIO_GO_TEST_TIMEOUT) ./cmd/... ./pkg/...

.PHONY: test-kafka-undockerized
test-kafka-undockerized: ensure-gopath
	go test -v -p 1 --timeout $(NUCLIO_GO_TEST_TIMEOUT) ./pkg/processor/trigger/kafka/...

# This is to work around hostname resolution issues for sarama and kafka in CI
.PHONY: test-periodic-undockerized
test-periodic-undockerized: ensure-gopath
	go test -v -p 1 --timeout $(NUCLIO_GO_TEST_TIMEOUT) $(shell go list ./cmd/... ./pkg/... | grep -v trigger/kafka)

.PHONY: test-k8s-undockerized
test-k8s-undockerized: ensure-gopath
	NUCLIO_K8S_TESTS_ENABLED=true go test -v -p 1 --timeout $(NUCLIO_GO_TEST_TIMEOUT) ./pkg/platform/kube/...

.PHONY: fmt
fmt:
	gofmt -s -w .

.PHONY: build-test
build-test: ensure-gopath build-base
	docker build \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--build-arg DOCKER_CLI_VERSION=$(DOCKER_CLI_VERSION) \
		--file $(NUCLIO_DOCKER_TEST_DOCKERFILE_PATH) \
		--tag $(NUCLIO_DOCKER_TEST_TAG) .

.PHONY: test
test: build-test
	docker run \
		--rm \
		--volume /var/run/docker.sock:/var/run/docker.sock \
		--volume $(GOPATH)/bin:/go/bin \
		--volume $(shell pwd):$(GO_BUILD_TOOL_WORKDIR) \
		--volume /tmp:/tmp \
		--workdir $(GO_BUILD_TOOL_WORKDIR) \
		--env NUCLIO_TEST_HOST=$(NUCLIO_TEST_HOST) \
		--env NUCLIO_VERSION_GIT_COMMIT=$(NUCLIO_VERSION_GIT_COMMIT) \
		--env NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--env NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--env NUCLIO_OS=$(NUCLIO_OS) \
		--env NUCLIO_GO_TEST_TIMEOUT=$(NUCLIO_GO_TEST_TIMEOUT) \
		$(NUCLIO_DOCKER_TEST_TAG) \
		/bin/bash -c "make test-undockerized"

.PHONY: test-k8s
test-k8s: build-test
	NUCLIO_TEST_KUBECONFIG=$(if $(NUCLIO_TEST_KUBECONFIG),$(NUCLIO_TEST_KUBECONFIG),$(KUBECONFIG)) \
	docker run \
		--rm \
		--network host \
		--volume /var/run/docker.sock:/var/run/docker.sock \
		--volume $(GOPATH)/bin:/go/bin \
		--volume $(shell pwd):$(GO_BUILD_TOOL_WORKDIR) \
		--volume /tmp:/tmp \
		--volume $(NUCLIO_TEST_KUBECONFIG)/:/kubeconfig \
		--workdir $(GO_BUILD_TOOL_WORKDIR) \
		--env NUCLIO_TEST_HOST=$(NUCLIO_TEST_HOST) \
		--env NUCLIO_VERSION_GIT_COMMIT=$(NUCLIO_VERSION_GIT_COMMIT) \
		--env NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--env NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--env NUCLIO_TEST_REGISTRY_URL=$(NUCLIO_TEST_REGISTRY_URL) \
		--env NUCLIO_OS=$(NUCLIO_OS) \
		--env MINIKUBE_HOME=$(MINIKUBE_HOME) \
		--env NUCLIO_GO_TEST_TIMEOUT=$(NUCLIO_GO_TEST_TIMEOUT) \
		--env KUBECONFIG=/kubeconfig \
		$(NUCLIO_DOCKER_TEST_TAG) \
		/bin/bash -c "make test-k8s-undockerized"

.PHONY: test-periodic
test-periodic: build-test
	docker run \
		--rm \
		--volume /var/run/docker.sock:/var/run/docker.sock \
		--volume $(GOPATH)/bin:/go/bin \
		--volume $(shell pwd):$(GO_BUILD_TOOL_WORKDIR) \
		--volume /tmp:/tmp \
		--workdir $(GO_BUILD_TOOL_WORKDIR) \
		--env NUCLIO_TEST_HOST=$(NUCLIO_TEST_HOST) \
		--env NUCLIO_VERSION_GIT_COMMIT=$(NUCLIO_VERSION_GIT_COMMIT) \
		--env NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--env NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--env NUCLIO_OS=$(NUCLIO_OS) \
		--env NUCLIO_GO_TEST_TIMEOUT=$(NUCLIO_GO_TEST_TIMEOUT) \
		$(NUCLIO_DOCKER_TEST_TAG) \
		/bin/bash -c "make test-periodic-undockerized"

.PHONY: test-kafka
test-kafka: build-test
	docker run \
		--rm \
		--volume /var/run/docker.sock:/var/run/docker.sock \
		--volume $(GOPATH)/bin:/go/bin \
		--volume $(shell pwd):$(GO_BUILD_TOOL_WORKDIR) \
		--volume /tmp:/tmp \
		--workdir $(GO_BUILD_TOOL_WORKDIR) \
		--env NUCLIO_TEST_HOST=$(NUCLIO_TEST_HOST) \
		--env NUCLIO_VERSION_GIT_COMMIT=$(NUCLIO_VERSION_GIT_COMMIT) \
		--env NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--env NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--env NUCLIO_OS=$(NUCLIO_OS) \
		--env NUCLIO_GO_TEST_TIMEOUT=$(NUCLIO_GO_TEST_TIMEOUT) \
		$(NUCLIO_DOCKER_TEST_TAG) \
		/bin/bash -c "make test-kafka-undockerized"

.PHONY: test-nodejs
test-nodejs:
	docker run \
	 --rm \
	 --volume $(shell pwd)/pkg/processor/runtime/nodejs/js:/nuclio/nodejs \
	 --volume $(shell pwd)/test:/nuclio/test \
	 --workdir /nuclio/nodejs \
	 --env RUN_MODE=CI \
	 node:10.20-alpine \
	 sh -c 'npm install && npm run lint && npm run test'

.PHONY: test-python
test-python:
	docker build \
		--build-arg CACHEBUST=$(shell date +%s) \
		--build-arg PYTHON_IMAGE_TAG=3.6-slim-stretch \
		-f pkg/processor/runtime/python/test/Dockerfile .
	docker build \
		--build-arg CACHEBUST=$(shell date +%s) \
		--build-arg PYTHON_IMAGE_TAG=2.7-slim-stretch \
		-f pkg/processor/runtime/python/test/Dockerfile .

.PHONY: test-short
test-short: modules ensure-gopath
	go test -v ./cmd/... ./pkg/... -short

.PHONY: test-k8s-nuctl
test-k8s-nuctl:
	NUCTL_EXTERNAL_IP_ADDRESSES=$(if $(NUCTL_EXTERNAL_IP_ADDRESSES),$(NUCTL_EXTERNAL_IP_ADDRESSES),"localhost") \
		NUCTL_RUN_REGISTRY=$(NUCTL_REGISTRY) \
		NUCTL_PLATFORM=kube \
		NUCTL_NAMESPACE=$(if $(NUCTL_NAMESPACE),$(NUCTL_NAMESPACE),"default") \
		go test -v github.com/nuclio/nuclio/pkg/nuctl/... -p 1 --timeout $(NUCLIO_GO_TEST_TIMEOUT)

.PHONY: test-docker-nuctl
test-docker-nuctl:
	NUCTL_PLATFORM=local go test -v github.com/nuclio/nuclio/pkg/nuctl/... -p 1 --timeout $(NUCLIO_GO_TEST_TIMEOUT)

.PHONY: build-base
build-base: build-builder
	docker build \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file hack/docker/build/base/Dockerfile \
		--tag nuclio-base:$(NUCLIO_LABEL) .
	docker build \
		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--file hack/docker/build/base-alpine/Dockerfile \
		--tag nuclio-base-alpine:$(NUCLIO_LABEL) .

.PHONY: build-builder
build-builder:
	docker build -f hack/docker/build/builder/Dockerfile -t nuclio-builder:$(NUCLIO_LABEL) .

.PHONY: ensure-gopath
ensure-gopath:
ifndef GOPATH
	$(error GOPATH must be set)
endif

.PHONY: targets
targets:
	awk -F: '/^[^ \t="]+:/ && !/PHONY/ {print $$1}' Makefile | sort -u
