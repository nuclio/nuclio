# Copyright 2023 The Nuclio Authors.
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
SHELL:=/bin/bash

# upstream repo
NUCLIO_DOCKER_REPO ?= quay.io/nuclio
NUCLIO_CACHE_REPO ?= ghcr.io/nuclio

# get default os / arch from go env
NUCLIO_DEFAULT_OS := $(shell go env GOOS)
ifeq ($(GOARCH), arm)
	NUCLIO_DEFAULT_ARCH := armhf
else ifeq ($(GOARCH), arm64)
	NUCLIO_DEFAULT_ARCH := arm64
else
	NUCLIO_DEFAULT_ARCH := $(shell go env GOARCH || echo amd64)
endif

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
NUCLIO_CACHE_LABEL := $(if $(NUCLIO_CACHE_LABEL),$(NUCLIO_CACHE_LABEL),unstable)

NUCLIO_TEST_HOST := $(if $(NUCLIO_TEST_HOST),$(NUCLIO_TEST_HOST),$(NUCLIO_DEFAULT_TEST_HOST))
NUCLIO_DEFAULT_EXTERNAL_IP_ADDRESS := "127.0.0.1"
NUCLIO_EXTERNAL_IP_ADDRESS := $(if $(NUCLIO_EXTERNAL_IP_ADDRESS),$(NUCLIO_EXTERNAL_IP_ADDRESS),$(NUCLIO_DEFAULT_EXTERNAL_IP_ADDRESS))
NUCLIO_VERSION_GIT_COMMIT = $(shell git rev-parse HEAD)
NUCLIO_PATH ?= $(shell pwd)

NUCLIO_VERSION_INFO = {\"git_commit\": \"$(NUCLIO_VERSION_GIT_COMMIT)\", \"label\": \"$(NUCLIO_LABEL)\"}

# Dockerized tests variables - not available for changes
NUCLIO_DOCKER_TEST_DOCKERFILE_PATH := test/docker/Dockerfile
NUCLIO_DOCKER_TEST_TAG := nuclio/tester

# Add labels to docker images
NUCLIO_DOCKER_LABELS = --label nuclio.version_info="$(NUCLIO_VERSION_INFO)"

NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_LABEL)-$(NUCLIO_ARCH)
NUCLIO_DOCKER_IMAGE_CACHE_TAG=$(NUCLIO_CACHE_LABEL)-$(NUCLIO_ARCH)-cache
NUCLIO_DOCKER_IMAGE_CACHE_ALPINE_TAG=$(NUCLIO_DOCKER_IMAGE_CACHE_TAG)-alpine

# Link flags
GO_LINK_FLAGS ?= -s -w
GO_LINK_FLAGS_INJECT_VERSION := $(GO_LINK_FLAGS) \
	-X github.com/v3io/version-go.gitCommit=$(NUCLIO_VERSION_GIT_COMMIT) \
	-X github.com/v3io/version-go.label=$(NUCLIO_LABEL) \
	-X github.com/v3io/version-go.arch=$(NUCLIO_ARCH)

# Nuclio test timeout
NUCLIO_GO_TEST_TIMEOUT ?= "60m"

NUCLIO_DEFAULT_LIST_TESTS_MAKE_COMMAND=list-all-dirs-with-tests
LIST_TESTS_MAKE_COMMAND := $(if $(LIST_TESTS_MAKE_COMMAND),$(LIST_TESTS_MAKE_COMMAND),$(NUCLIO_DEFAULT_LIST_TESTS_MAKE_COMMAND))

# Docker client cli to be used
NUCLIO_DOCKER_CLIENT_VERSION ?= 23.0.1
ifeq ($(NUCLIO_ARCH), armhf)
	NUCLIO_DOCKER_CLIENT_ARCH ?= armhf
else ifeq ($(NUCLIO_ARCH), arm64)
	NUCLIO_DOCKER_CLIENT_ARCH ?= aarch64
else
	NUCLIO_DOCKER_CLIENT_ARCH ?= x86_64
endif

# alpine is commonly used by controller / dlx / autoscaler
ifeq ($(NUCLIO_ARCH), armhf)
	NUCLIO_DOCKER_ALPINE_IMAGE 		?= gcr.io/iguazio/arm32v7/alpine:3.17
	NUCLIO_BASE_IMAGE_NAME 			?= gcr.io/iguazio/arm32v7/golang
	NUCLIO_DOCKER_JAVA_OPENJDK		?= gcr.io/iguazio/openjdk:11-slim
	NODE_IMAGE_NAME 				?= gcr.io/iguazio/arm32v7/node:14.21
else ifeq ($(NUCLIO_ARCH), arm64)
	NUCLIO_DOCKER_ALPINE_IMAGE 		?= gcr.io/iguazio/arm64v8/alpine:3.17
	NUCLIO_BASE_IMAGE_NAME 			?= gcr.io/iguazio/arm64v8/golang
	NUCLIO_DOCKER_JAVA_OPENJDK 		?= gcr.io/iguazio/arm64v8/openjdk:11-slim
	NODE_IMAGE_NAME 				?= gcr.io/iguazio/arm64v8/node:14.21
else
	NUCLIO_DOCKER_ALPINE_IMAGE 		?= gcr.io/iguazio/alpine:3.17
	NUCLIO_BASE_IMAGE_NAME 			?= gcr.io/iguazio/golang
	NUCLIO_DOCKER_JAVA_OPENJDK		?= gcr.io/iguazio/openjdk:11-slim
	NODE_IMAGE_NAME 				?= gcr.io/iguazio/node:14.21
endif

NUCLIO_BASE_IMAGE_TAG ?= 1.21
NUCLIO_BASE_ALPINE_IMAGE_TAG ?= 1.21-alpine

#
#  Must be first target
#
.PHONY: all
all:
	$(error "Please pick a target (run 'make targets' to view targets)")


#
# Version resources
#
.PHONY: helm-publish
helm-publish:
	$(eval HELM_PUBLISH_COMMIT_MESSAGE := "Releasing chart $(shell helm inspect chart hack/k8s/helm/nuclio | yq eval '.version' -)")
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
GO_BUILD_CMD = go build -ldflags="$(GO_LINK_FLAGS_INJECT_VERSION)"

#
# Rules
#

.PHONY: build
build: docker-images tools
	@echo Done.

DOCKER_IMAGES_RULES ?= \
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

DOCKER_IMAGES_CACHE ?=


.PHONY: docker-images
docker-images: $(DOCKER_IMAGES_RULES)
	@echo Done.

.PHONY: pull-docker-images-cache
pull-docker-images-cache:
	@printf '%s\n' $(DOCKER_IMAGES_CACHE) | xargs -n 1 -P 5 docker pull

.PHONY: push-docker-images-cache
push-docker-images-cache:
	@printf '%s\n' $(DOCKER_IMAGES_CACHE) | xargs -n 1 -P 5 docker push

.PHONY: tools
tools: ensure-gopath nuctl
	@echo Done.

.PHONY: push-docker-images
push-docker-images: print-docker-images
	@echo "Pushing images concurrently"
	@echo $(IMAGES_TO_PUSH) | xargs -n 1 -P 5 docker push
	@echo Done.

.PHONY: save-docker-images
save-docker-images: print-docker-images
	@echo "Saving Nuclio docker images"
	docker save $(IMAGES_TO_PUSH) | gzip --fast > nuclio-docker-images-$(NUCLIO_LABEL)-$(NUCLIO_ARCH).tar.gz

.PHONY: load-docker-images
load-docker-images: print-docker-images
	@echo "Load Nuclio docker images"
	docker load -i nuclio-docker-images-$(NUCLIO_LABEL)-$(NUCLIO_ARCH).tar.gz

.PHONY: print-docker-images
pull-docker-images: print-docker-images
	@echo "Pull Nuclio docker images"
	@echo $(IMAGES_TO_PUSH) | xargs -n 1 -P 5 docker pull

.PHONY: retag-docker-images
retag-docker-images: print-docker-images
	$(eval NUCLIO_NEW_LABEL ?= retagged)
	$(eval NUCLIO_NEW_LABEL = ${NUCLIO_NEW_LABEL}-${NUCLIO_ARCH})
	@echo "Retagging Nuclio docker images with ${NUCLIO_NEW_LABEL}"
	echo $(IMAGES_TO_PUSH) | xargs -P 5 -I{} sh -c 'image="{}"; docker tag $$image $$(echo $$image | cut -d : -f 1):$(NUCLIO_NEW_LABEL)'
	@echo "Done"

.PHONY: print-docker-images
print-docker-images:
	@# env to determine whether to print only first image
	$(eval PRINT_FIRST_IMAGE ?= false)
	@for image in $(IMAGES_TO_PUSH); do \
		echo $$image ; \
		if [ "$(PRINT_FIRST_IMAGE)" = "true" ]; then \
			break ; \
		fi ; \
	done


.PHONY: print-docker-images-cache
print-docker-images-cache:
	@echo "Nuclio Docker images cache:"
	@for image in $(DOCKER_IMAGES_CACHE); do \
		echo $$image; \
	done


.PHONY: print-docker-image-rules-json
print-docker-image-rules-json:
	@/bin/echo -n "["
	@for image in $(DOCKER_IMAGES_RULES); do \
		/bin/echo -n "{\"image_rule\": \"$$image\"}" ; \
		if [ "$$image" != "$(lastword $(DOCKER_IMAGES_RULES))" ]; then \
			/bin/echo -n "," ; \
		fi ; \
	done
	@/bin/echo -n "]"

#
# Tools
#

NUCLIO_NUCTL_CREATE_SYMLINK := $(if $(NUCLIO_NUCTL_CREATE_SYMLINK),$(NUCLIO_NUCTL_CREATE_SYMLINK),true)
NUCTL_BIN_NAME = nuctl-$(NUCLIO_LABEL)-$(NUCLIO_OS)-$(NUCLIO_ARCH)
NUCTL_TARGET = $(GOPATH)/bin/nuctl

.PHONY: nuctl
nuctl: ensure-gopath build-builder
	docker run \
		--volume $(GOPATH)/bin:/go/bin \
		--env GOOS=$(NUCLIO_OS) \
		--env GOARCH=$(NUCLIO_ARCH) \
		$(NUCLIO_DOCKER_REPO)/nuclio-builder:$(NUCLIO_DOCKER_IMAGE_TAG) \
		$(GO_BUILD_CMD) -o /go/bin/$(NUCTL_BIN_NAME) cmd/nuctl/main.go
ifeq ($(NUCLIO_NUCTL_CREATE_SYMLINK), true)
	@rm -f $(NUCTL_TARGET)
	@ln -sF $(GOPATH)/bin/$(NUCTL_BIN_NAME) $(NUCTL_TARGET)
endif

.PHONY: nuctl-bin
nuctl-bin: ensure-gopath
	CGO_ENABLED=0 $(GO_BUILD_CMD) -o $(NUCLIO_PATH)/$(NUCTL_BIN_NAME) cmd/nuctl/main.go

NUCLIO_DOCKER_PROCESSOR_IMAGE_NAME=$(NUCLIO_DOCKER_REPO)/processor:$(NUCLIO_DOCKER_IMAGE_TAG)
NUCLIO_DOCKER_PROCESSOR_IMAGE_NAME_CACHE=$(NUCLIO_CACHE_REPO)/processor:$(NUCLIO_DOCKER_IMAGE_CACHE_TAG)

.PHONY: processor
processor: modules

	@# build processor locally
	@# build its image and copy from host to image
	@# this is done to avoid trying compiling the processor binary on the image
	@# while using virtualization / emulation to match the desired architecture
	@mkdir -p ./.bin
	GOARCH=$(NUCLIO_ARCH) GOOS=linux CGO_ENABLED=0 $(GO_BUILD_CMD) \
        -o ./.bin/processor-$(NUCLIO_ARCH) \
        cmd/processor/main.go

	docker build \
		--build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		--cache-from $(NUCLIO_DOCKER_PROCESSOR_IMAGE_NAME_CACHE) \
		--file cmd/processor/Dockerfile \
		--tag $(NUCLIO_DOCKER_PROCESSOR_IMAGE_NAME) \
		--tag $(NUCLIO_DOCKER_PROCESSOR_IMAGE_NAME_CACHE) \
		.

ifneq ($(filter processor,$(DOCKER_IMAGES_RULES)),)
$(eval IMAGES_TO_PUSH += $(NUCLIO_DOCKER_PROCESSOR_IMAGE_NAME))
$(eval DOCKER_IMAGES_CACHE += $(NUCLIO_DOCKER_PROCESSOR_IMAGE_NAME_CACHE))
endif

#
# Dockerized services
#

# Controller
NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME=$(NUCLIO_DOCKER_REPO)/controller:$(NUCLIO_DOCKER_IMAGE_TAG)
NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME_CACHE=$(NUCLIO_CACHE_REPO)/controller:$(NUCLIO_DOCKER_IMAGE_CACHE_TAG)

.PHONY: controller
controller: build-builder
	docker build \
		--build-arg ALPINE_IMAGE=$(NUCLIO_DOCKER_ALPINE_IMAGE) \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_DOCKER_IMAGE_TAG) \
		--build-arg NUCLIO_DOCKER_REPO=$(NUCLIO_DOCKER_REPO) \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		--cache-from $(NUCLIO_CACHE_REPO)/controller:$(NUCLIO_DOCKER_IMAGE_CACHE_TAG) \
		--file cmd/controller/Dockerfile \
		--tag $(NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME) \
		--tag $(NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME_CACHE) \
		$(NUCLIO_DOCKER_LABELS) .

ifneq ($(filter controller,$(DOCKER_IMAGES_RULES)),)
$(eval IMAGES_TO_PUSH 		+= $(NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME))
$(eval DOCKER_IMAGES_CACHE 	+= $(NUCLIO_DOCKER_CONTROLLER_IMAGE_NAME_CACHE))
endif

# Dashboard
NUCLIO_DOCKER_DASHBOARD_IMAGE_NAME    		= $(NUCLIO_DOCKER_REPO)/dashboard:$(NUCLIO_DOCKER_IMAGE_TAG)
NUCLIO_DOCKER_DASHBOARD_IMAGE_NAME_CACHE    = $(NUCLIO_CACHE_REPO)/dashboard:$(NUCLIO_DOCKER_IMAGE_CACHE_TAG)
NUCLIO_DOCKER_DASHBOARD_UHTTPC_ARCH  		?= $(NUCLIO_ARCH)

ifeq ($(NUCLIO_ARCH), armhf)
	NUCLIO_DOCKER_DASHBOARD_NGINX_BASE_IMAGE  ?= gcr.io/iguazio/arm32v7/nginx:1.25-alpine
else ifeq ($(NUCLIO_ARCH), arm64)
	NUCLIO_DOCKER_DASHBOARD_NGINX_BASE_IMAGE  ?= gcr.io/iguazio/arm64v8/nginx:1.25-alpine
else
	NUCLIO_DOCKER_DASHBOARD_NGINX_BASE_IMAGE  ?= gcr.io/iguazio/nginx:1.25-alpine
endif

.PHONY: dashboard
dashboard: build-builder
	docker build \
		--build-arg DOCKER_CLI_ARCH=$(NUCLIO_DOCKER_CLIENT_ARCH) \
		--build-arg DOCKER_CLI_VERSION=$(NUCLIO_DOCKER_CLIENT_VERSION) \
		--build-arg NGINX_IMAGE=$(NUCLIO_DOCKER_DASHBOARD_NGINX_BASE_IMAGE) \
		--build-arg NUCLIO_DOCKER_ALPINE_IMAGE=$(NUCLIO_DOCKER_ALPINE_IMAGE) \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--build-arg UHTTPC_ARCH="$(NUCLIO_DOCKER_DASHBOARD_UHTTPC_ARCH)" \
		--build-arg NODE_IMAGE_NAME=$(NODE_IMAGE_NAME) \
		--build-arg NUCLIO_DOCKER_REPO=$(NUCLIO_DOCKER_REPO) \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_DOCKER_IMAGE_TAG) \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		--cache-from $(NUCLIO_DOCKER_DASHBOARD_IMAGE_NAME_CACHE) \
		--file cmd/dashboard/docker/Dockerfile \
		--tag $(NUCLIO_DOCKER_DASHBOARD_IMAGE_NAME) \
		--tag $(NUCLIO_DOCKER_DASHBOARD_IMAGE_NAME_CACHE) \
		$(NUCLIO_DOCKER_LABELS) .

ifneq ($(filter dashboard,$(DOCKER_IMAGES_RULES)),)
$(eval IMAGES_TO_PUSH 		+= $(NUCLIO_DOCKER_DASHBOARD_IMAGE_NAME))
$(eval DOCKER_IMAGES_CACHE 	+= $(NUCLIO_DOCKER_DASHBOARD_IMAGE_NAME_CACHE))
endif

# Scaler
NUCLIO_DOCKER_SCALER_IMAGE_NAME=$(NUCLIO_DOCKER_REPO)/autoscaler:$(NUCLIO_DOCKER_IMAGE_TAG)
NUCLIO_DOCKER_SCALER_IMAGE_NAME_CACHE=$(NUCLIO_CACHE_REPO)/autoscaler:$(NUCLIO_DOCKER_IMAGE_CACHE_TAG)

.PHONY: autoscaler
autoscaler: build-builder
	docker build \
		--build-arg ALPINE_IMAGE=$(NUCLIO_DOCKER_ALPINE_IMAGE) \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_DOCKER_IMAGE_TAG) \
		--build-arg NUCLIO_DOCKER_REPO=$(NUCLIO_DOCKER_REPO) \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		--cache-from $(NUCLIO_DOCKER_SCALER_IMAGE_NAME_CACHE) \
		--file cmd/autoscaler/Dockerfile \
		--tag $(NUCLIO_DOCKER_SCALER_IMAGE_NAME) \
		--tag $(NUCLIO_DOCKER_SCALER_IMAGE_NAME_CACHE) \
		$(NUCLIO_DOCKER_LABELS) .

ifneq ($(filter autoscaler,$(DOCKER_IMAGES_RULES)),)
$(eval IMAGES_TO_PUSH += $(NUCLIO_DOCKER_SCALER_IMAGE_NAME))
$(eval DOCKER_IMAGES_CACHE += $(NUCLIO_DOCKER_SCALER_IMAGE_NAME_CACHE))
endif

# Dlx
NUCLIO_DOCKER_DLX_IMAGE_NAME=$(NUCLIO_DOCKER_REPO)/dlx:$(NUCLIO_DOCKER_IMAGE_TAG)
NUCLIO_DOCKER_DLX_IMAGE_NAME_CACHE=$(NUCLIO_CACHE_REPO)/dlx:$(NUCLIO_DOCKER_IMAGE_CACHE_TAG)

.PHONY: dlx
dlx: build-builder
	docker build \
		--build-arg ALPINE_IMAGE=$(NUCLIO_DOCKER_ALPINE_IMAGE) \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_DOCKER_IMAGE_TAG) \
		--build-arg NUCLIO_DOCKER_REPO=$(NUCLIO_DOCKER_REPO) \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		--cache-from $(NUCLIO_DOCKER_DLX_IMAGE_NAME_CACHE) \
		--file cmd/dlx/Dockerfile \
		--tag $(NUCLIO_DOCKER_DLX_IMAGE_NAME) \
		--tag $(NUCLIO_DOCKER_DLX_IMAGE_NAME_CACHE) \
		$(NUCLIO_DOCKER_LABELS) .

ifneq ($(filter dlx,$(DOCKER_IMAGES_RULES)),)
$(eval IMAGES_TO_PUSH += $(NUCLIO_DOCKER_DLX_IMAGE_NAME))
$(eval DOCKER_IMAGES_CACHE += $(NUCLIO_DOCKER_DLX_IMAGE_NAME_CACHE))
endif

#
# Onbuild images
#

# Python
NUCLIO_DOCKER_HANDLER_BUILDER_PYTHON_ONBUILD_IMAGE_NAME=\
 $(NUCLIO_DOCKER_REPO)/handler-builder-python-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)
NUCLIO_DOCKER_HANDLER_BUILDER_PYTHON_ONBUILD_IMAGE_NAME_CACHE=\
 $(NUCLIO_CACHE_REPO)/handler-builder-python-onbuild:$(NUCLIO_DOCKER_IMAGE_CACHE_TAG)

PIP_REQUIRE_VIRTUALENV=false

.PHONY: handler-builder-python-onbuild
handler-builder-python-onbuild: processor
	docker build \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_DOCKER_IMAGE_TAG) \
		--build-arg NUCLIO_DOCKER_REPO=$(NUCLIO_DOCKER_REPO) \
		--cache-from $(NUCLIO_DOCKER_HANDLER_BUILDER_PYTHON_ONBUILD_IMAGE_NAME_CACHE) \
		--file pkg/processor/build/runtime/python/docker/onbuild/Dockerfile \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_PYTHON_ONBUILD_IMAGE_NAME) \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_PYTHON_ONBUILD_IMAGE_NAME_CACHE) \
		.

ifneq ($(filter handler-builder-python-onbuild,$(DOCKER_IMAGES_RULES)),)
$(eval IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_PYTHON_ONBUILD_IMAGE_NAME))
$(eval DOCKER_IMAGES_CACHE += $(NUCLIO_DOCKER_HANDLER_BUILDER_PYTHON_ONBUILD_IMAGE_NAME_CACHE))
endif

# Go
NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME=\
 $(NUCLIO_DOCKER_REPO)/handler-builder-golang-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)

NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME=\
 $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME)-alpine

NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME_CACHE=\
 $(NUCLIO_CACHE_REPO)/handler-builder-golang-onbuild:$(NUCLIO_DOCKER_IMAGE_CACHE_TAG)

NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME_CACHE=\
 $(NUCLIO_CACHE_REPO)/handler-builder-golang-onbuild:$(NUCLIO_DOCKER_IMAGE_CACHE_ALPINE_TAG)

.PHONY: handler-builder-golang-onbuild-alpine
handler-builder-golang-onbuild-alpine: build-builder
	docker build \
		--build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--build-arg NUCLIO_BASE_IMAGE_NAME=$(NUCLIO_BASE_IMAGE_NAME) \
		--build-arg NUCLIO_BASE_ALPINE_IMAGE_TAG=$(NUCLIO_BASE_ALPINE_IMAGE_TAG) \
		--cache-from $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME_CACHE) \
		--file pkg/processor/build/runtime/golang/docker/onbuild/Dockerfile.alpine \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME) \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME_CACHE) \
		.

.PHONY: handler-builder-golang-onbuild
handler-builder-golang-onbuild: build-builder
ifndef SKIP_BUILD_GOLANG_ONBUILD_ALPINE
handler-builder-golang-onbuild: handler-builder-golang-onbuild-alpine
endif
handler-builder-golang-onbuild:
	docker build \
		--build-arg NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_DOCKER_IMAGE_TAG) \
		--build-arg NUCLIO_GO_LINK_FLAGS_INJECT_VERSION="$(GO_LINK_FLAGS_INJECT_VERSION)" \
		--build-arg NUCLIO_BASE_IMAGE_NAME=$(NUCLIO_BASE_IMAGE_NAME) \
		--build-arg NUCLIO_BASE_IMAGE_TAG=$(NUCLIO_BASE_IMAGE_TAG) \
		--build-arg NUCLIO_DOCKER_REPO=$(NUCLIO_DOCKER_REPO) \
		--cache-from $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME_CACHE) \
		--file pkg/processor/build/runtime/golang/docker/onbuild/Dockerfile \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME) \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME_CACHE) \
		.

ifneq ($(filter handler-builder-golang-onbuild,$(DOCKER_IMAGES_RULES)),)
$(eval IMAGES_TO_PUSH 		+= $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME))
$(eval DOCKER_IMAGES_CACHE 	+= $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_IMAGE_NAME_CACHE))
ifndef SKIP_BUILD_GOLANG_ONBUILD_ALPINE
$(eval IMAGES_TO_PUSH 		+= $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME))
$(eval DOCKER_IMAGES_CACHE	+= $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME_CACHE))
endif
endif

ifneq ($(filter handler-builder-golang-onbuild-alpine,$(DOCKER_IMAGES_RULES)),)
$(eval IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME))
$(eval DOCKER_IMAGES_CACHE += $(NUCLIO_DOCKER_HANDLER_BUILDER_GOLANG_ONBUILD_ALPINE_IMAGE_NAME_CACHE))
endif

# NodeJS
NUCLIO_DOCKER_HANDLER_BUILDER_NODEJS_ONBUILD_IMAGE_NAME=\
 $(NUCLIO_DOCKER_REPO)/handler-builder-nodejs-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)

NUCLIO_DOCKER_HANDLER_BUILDER_NODEJS_ONBUILD_IMAGE_NAME_CACHE=\
 $(NUCLIO_CACHE_REPO)/handler-builder-nodejs-onbuild:$(NUCLIO_DOCKER_IMAGE_CACHE_TAG)

.PHONY: handler-builder-nodejs-onbuild
handler-builder-nodejs-onbuild: processor
	docker build \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_DOCKER_IMAGE_TAG) \
		--build-arg NUCLIO_DOCKER_REPO=$(NUCLIO_DOCKER_REPO) \
		--file pkg/processor/build/runtime/nodejs/docker/onbuild/Dockerfile \
		--cache-from $(NUCLIO_DOCKER_HANDLER_BUILDER_NODEJS_ONBUILD_IMAGE_NAME_CACHE) \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_NODEJS_ONBUILD_IMAGE_NAME) \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_NODEJS_ONBUILD_IMAGE_NAME_CACHE) \
		.

ifneq ($(filter handler-builder-nodejs-onbuild,$(DOCKER_IMAGES_RULES)),)
$(eval IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_NODEJS_ONBUILD_IMAGE_NAME))
$(eval DOCKER_IMAGES_CACHE += $(NUCLIO_DOCKER_HANDLER_BUILDER_NODEJS_ONBUILD_IMAGE_NAME_CACHE))
endif

# Ruby
NUCLIO_DOCKER_HANDLER_BUILDER_RUBY_ONBUILD_IMAGE_NAME=\
 $(NUCLIO_DOCKER_REPO)/handler-builder-ruby-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)

NUCLIO_DOCKER_HANDLER_BUILDER_RUBY_ONBUILD_IMAGE_NAME_CACHE=\
 $(NUCLIO_CACHE_REPO)/handler-builder-ruby-onbuild:$(NUCLIO_DOCKER_IMAGE_CACHE_TAG)

.PHONY: handler-builder-ruby-onbuild
handler-builder-ruby-onbuild: processor
	docker build \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_DOCKER_IMAGE_TAG) \
		--build-arg NUCLIO_DOCKER_REPO=$(NUCLIO_DOCKER_REPO) \
		--file pkg/processor/build/runtime/ruby/docker/onbuild/Dockerfile \
		--cache-from $(NUCLIO_DOCKER_HANDLER_BUILDER_RUBY_ONBUILD_IMAGE_NAME_CACHE) \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_RUBY_ONBUILD_IMAGE_NAME) \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_RUBY_ONBUILD_IMAGE_NAME_CACHE) \
		.

ifneq ($(filter handler-builder-ruby-onbuild,$(DOCKER_IMAGES_RULES)),)
$(eval IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_RUBY_ONBUILD_IMAGE_NAME))
$(eval DOCKER_IMAGES_CACHE += $(NUCLIO_DOCKER_HANDLER_BUILDER_RUBY_ONBUILD_IMAGE_NAME_CACHE))
endif


# .NetCore
NUCLIO_DOCKER_HANDLER_BUILDER_DOTNETCORE_ONBUILD_IMAGE_NAME=\
 $(NUCLIO_DOCKER_REPO)/handler-builder-dotnetcore-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)
NUCLIO_DOCKER_HANDLER_BUILDER_DOTNETCORE_ONBUILD_IMAGE_NAME_CACHE=\
 $(NUCLIO_CACHE_REPO)/handler-builder-dotnetcore-onbuild:$(NUCLIO_DOCKER_IMAGE_CACHE_TAG)
NUCLIO_ONBUILD_DOTNETCORE_DOCKERFILE_PATH = pkg/processor/build/runtime/dotnetcore/docker/onbuild/Dockerfile

.PHONY: handler-builder-dotnetcore-onbuild
handler-builder-dotnetcore-onbuild: processor
	docker build \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_DOCKER_IMAGE_TAG) \
		--build-arg NUCLIO_DOCKER_REPO=$(NUCLIO_DOCKER_REPO) \
		--cache-from $(NUCLIO_DOCKER_HANDLER_BUILDER_DOTNETCORE_ONBUILD_IMAGE_NAME_CACHE) \
		--file $(NUCLIO_ONBUILD_DOTNETCORE_DOCKERFILE_PATH) \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_DOTNETCORE_ONBUILD_IMAGE_NAME) \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_DOTNETCORE_ONBUILD_IMAGE_NAME_CACHE) \
		.

ifneq ($(filter handler-builder-dotnetcore-onbuild,$(DOCKER_IMAGES_RULES)),)
$(eval IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_DOTNETCORE_ONBUILD_IMAGE_NAME))
$(eval DOCKER_IMAGES_CACHE += $(NUCLIO_DOCKER_HANDLER_BUILDER_DOTNETCORE_ONBUILD_IMAGE_NAME_CACHE))
endif

# Java
NUCLIO_DOCKER_HANDLER_BUILDER_JAVA_ONBUILD_IMAGE_NAME=\
 $(NUCLIO_DOCKER_REPO)/handler-builder-java-onbuild:$(NUCLIO_DOCKER_IMAGE_TAG)
NUCLIO_DOCKER_HANDLER_BUILDER_JAVA_ONBUILD_IMAGE_NAME_CACHE=\
 $(NUCLIO_CACHE_REPO)/handler-builder-java-onbuild:$(NUCLIO_DOCKER_IMAGE_CACHE_TAG)

.PHONY: handler-builder-java-onbuild
handler-builder-java-onbuild: processor
	docker build \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_DOCKER_IMAGE_TAG) \
		--build-arg NUCLIO_DOCKER_REPO=$(NUCLIO_DOCKER_REPO) \
		--build-arg NUCLIO_DOCKER_JAVA_OPENJDK=$(NUCLIO_DOCKER_JAVA_OPENJDK) \
		--cache-from $(NUCLIO_DOCKER_HANDLER_BUILDER_JAVA_ONBUILD_IMAGE_NAME_CACHE) \
		--file pkg/processor/build/runtime/java/docker/onbuild/Dockerfile \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_JAVA_ONBUILD_IMAGE_NAME) \
		--tag $(NUCLIO_DOCKER_HANDLER_BUILDER_JAVA_ONBUILD_IMAGE_NAME_CACHE) \
		.

ifneq ($(filter handler-builder-java-onbuild,$(DOCKER_IMAGES_RULES)),)
$(eval IMAGES_TO_PUSH += $(NUCLIO_DOCKER_HANDLER_BUILDER_JAVA_ONBUILD_IMAGE_NAME))
$(eval DOCKER_IMAGES_CACHE += $(NUCLIO_DOCKER_HANDLER_BUILDER_JAVA_ONBUILD_IMAGE_NAME_CACHE))
endif

NUCLIO_DOCKER_BUILDER_IMAGE_NAME=\
 $(NUCLIO_DOCKER_REPO)/nuclio-builder:$(NUCLIO_DOCKER_IMAGE_TAG)
NUCLIO_DOCKER_BUILDER_IMAGE_NAME_CACHE=\
 $(NUCLIO_CACHE_REPO)/nuclio-builder:$(NUCLIO_DOCKER_IMAGE_CACHE_TAG)

.PHONY: build-builder
build-builder:
	docker build \
		--build-arg NUCLIO_BASE_IMAGE_NAME=$(NUCLIO_BASE_IMAGE_NAME) \
		--build-arg NUCLIO_BASE_IMAGE_TAG=$(NUCLIO_BASE_IMAGE_TAG) \
		--build-arg BUILDKIT_INLINE_CACHE=1 \
		--cache-from $(NUCLIO_DOCKER_BUILDER_IMAGE_NAME_CACHE) \
		--file hack/docker/build/builder/Dockerfile \
		--tag $(NUCLIO_DOCKER_BUILDER_IMAGE_NAME) \
		--tag $(NUCLIO_DOCKER_BUILDER_IMAGE_NAME_CACHE) \
		.

$(eval DOCKER_IMAGES_CACHE += $(filter-out $(DOCKER_IMAGES_CACHE),$(NUCLIO_DOCKER_BUILDER_IMAGE_NAME_CACHE)))


#
# Misc
#

.PHONY: fmt
fmt:
	gofmt -s -w .
	golangci-lint run --fix

.PHONY: lint
lint: modules ensure-test-files-annotated
	@echo Installing linters...
	@test -e $(GOPATH)/bin/golangci-lint || \
	  	(curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin v1.54.2)

	@echo Linting...
	$(GOPATH)/bin/golangci-lint run -v
	@echo Done.

.PHONY: ensure-test-files-annotated
ensure-test-files-annotated:
	$(eval test_files_missing_build_annotations=$(strip $(shell find . -type f -name '*_test.go' -exec bash -c "grep -m 1 -L '//go:build ' {} | grep go" \;)))
	@if [ -n "$(test_files_missing_build_annotations)" ]; then \
		echo "Found go test files without build annotations: "; \
		echo $(test_files_missing_build_annotations); \
		echo "!!! Go test files must be annotated with '//go:build test_<x>' !!!"; \
		exit 1; \
	fi
	@echo "All go test files have //go:build test_X annotation"
	@exit $(.SHELLSTATUS)

#
# Testing
#

.PHONY: benchmarking
benchmarking:
	$(eval NUCLIO_BENCHMARKING_RUNTIMES ?= all)
	@python3 hack/scripts/benchmark/benchmark.py --nuctl-platform local --runtimes $(NUCLIO_BENCHMARKING_RUNTIMES)

.PHONY: functiontemplates
functiontemplates: modules ensure-gopath
	go run -tags=function_templates_generator pkg/dashboard/functiontemplates/generator/generator.go

.PHONY: generate-crds
generate-crds: build-builder
	docker build \
		--file hack/scripts/generate-crds/Dockerfile \
 		--build-arg NUCLIO_LABEL=$(NUCLIO_LABEL) \
 		--build-arg NUCLIO_DOCKER_REPO=$(NUCLIO_DOCKER_REPO) \
 		--tag nuclio/crds:latest .
	docker run \
		--rm \
		--volume $(shell pwd)/hack:/nuclio/hack \
		--volume $(shell pwd)/pkg:/output \
		--workdir /nuclio \
		nuclio/crds:latest

.PHONY: test-unit
test-unit: modules ensure-gopath
	go test -race -tags=test_unit -v ./cmd/... ./pkg/... -short

.PHONY: test-k8s-nuctl
test-k8s-nuctl:
	NUCTL_EXTERNAL_IP_ADDRESSES=$(if $(NUCTL_EXTERNAL_IP_ADDRESSES),$(NUCTL_EXTERNAL_IP_ADDRESSES),"localhost") \
		NUCTL_RUN_REGISTRY=$(NUCTL_REGISTRY) \
		NUCTL_PLATFORM=kube \
		NUCTL_NAMESPACE=$(if $(NUCTL_NAMESPACE),$(NUCTL_NAMESPACE),"default") \
		go test -tags="test_integration,test_kube" -v github.com/nuclio/nuclio/pkg/nuctl/... -p 1 --timeout $(NUCLIO_GO_TEST_TIMEOUT)

.PHONY: test-docker-nuctl
test-docker-nuctl:
	NUCTL_PLATFORM=local \
		go test -tags="test_integration,test_local" -v github.com/nuclio/nuclio/pkg/nuctl/... -p 1 --timeout $(NUCLIO_GO_TEST_TIMEOUT)

.PHONY: test-undockerized
test-undockerized: ensure-gopath
	${eval LIST=${shell make --no-print-directory $(LIST_TESTS_MAKE_COMMAND)}}
	go test  \
		-tags="test_integration,test_local" \
		-v \
		-p 1 \
		--timeout $(NUCLIO_GO_TEST_TIMEOUT) \
		${LIST}

.PHONY: test-k8s-undockerized
test-k8s-undockerized: ensure-gopath
	@# nuctl is running by "test-k8s-nuctl" target and requires specific set of env
	go test \
		-tags="test_integration,test_kube" \
 		-v \
 		-p 1 \
 		--timeout $(NUCLIO_GO_TEST_TIMEOUT) \
 		$(shell go list -tags="test_integration,test_kube" ./cmd/... ./pkg/... | grep -v nuctl)

.PHONY: test-broken-undockerized
test-broken-undockerized: ensure-gopath
	${eval LIST=${shell make --no-print-directory $(LIST_TESTS_MAKE_COMMAND)}}
	go test  \
		-tags="test_integration,test_broken" \
		-v \
		-p 1 \
		--timeout $(NUCLIO_GO_TEST_TIMEOUT) \
		${LIST}

.PHONY: test
test: build-test
	$(eval NUCLIO_TEST_MAKE_TARGET ?= $(if $(NUCLIO_TEST_BROKEN),test-broken-undockerized,test-undockerized))
	@docker run \
		--rm \
		--volume /var/run/docker.sock:/var/run/docker.sock \
		--volume $(GOPATH)/bin:/go/bin \
		--volume $(NUCLIO_PATH):$(GO_BUILD_TOOL_WORKDIR) \
		--volume /tmp:/tmp \
		--workdir $(GO_BUILD_TOOL_WORKDIR) \
		--env NUCLIO_TEST_HOST=$(NUCLIO_TEST_HOST) \
		--env NUCLIO_VERSION_GIT_COMMIT=$(NUCLIO_VERSION_GIT_COMMIT) \
		--env NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--env NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--env NUCLIO_OS=$(NUCLIO_OS) \
		--env NUCLIO_GO_TEST_TIMEOUT=$(NUCLIO_GO_TEST_TIMEOUT) \
		--env NUCLIO_TEST_HOST_PATH=$(NUCLIO_PATH) \
		--env NUCLIO_CI_SKIP_STRESS_TEST \
		$(NUCLIO_DOCKER_TEST_TAG) \
		/bin/bash -c "git config --global --add safe.directory /nuclio && LIST_TESTS_MAKE_COMMAND=${LIST_TESTS_MAKE_COMMAND} make ${NUCLIO_TEST_MAKE_TARGET}"
.PHONY: test-k8s
test-k8s: build-test
	NUCLIO_TEST_KUBECONFIG=$(if $(NUCLIO_TEST_KUBECONFIG),$(NUCLIO_TEST_KUBECONFIG),$(KUBECONFIG)) \
	docker run \
		--rm \
		--network host \
		--volume /var/run/docker.sock:/var/run/docker.sock \
		--volume $(GOPATH)/bin:/go/bin \
		--volume $(NUCLIO_PATH):$(GO_BUILD_TOOL_WORKDIR) \
		--volume /tmp:/tmp \
		--volume $(NUCLIO_TEST_KUBECONFIG)/:/kubeconfig \
		--workdir $(GO_BUILD_TOOL_WORKDIR) \
		--env NUCLIO_TEST_HOST=$(NUCLIO_TEST_HOST) \
		--env NUCLIO_EXTERNAL_IP_ADDRESS=$(NUCLIO_EXTERNAL_IP_ADDRESS) \
		--env NUCLIO_VERSION_GIT_COMMIT=$(NUCLIO_VERSION_GIT_COMMIT) \
		--env NUCLIO_LABEL=$(NUCLIO_LABEL) \
		--env NUCLIO_ARCH=$(NUCLIO_ARCH) \
		--env NUCLIO_TEST_REGISTRY_URL=$(NUCLIO_TEST_REGISTRY_URL) \
		--env NUCLIO_OS=$(NUCLIO_OS) \
		--env MINIKUBE_HOME=$(MINIKUBE_HOME) \
		--env NUCLIO_GO_TEST_TIMEOUT=$(NUCLIO_GO_TEST_TIMEOUT) \
		--env KUBECONFIG=/kubeconfig \
		--env NUCLIO_TEST_KUBE_DEFAULT_INGRESS_HOST=$(NUCLIO_TEST_KUBE_DEFAULT_INGRESS_HOST) \
		$(NUCLIO_DOCKER_TEST_TAG) \
		/bin/bash -c "git config --global --add safe.directory /nuclio && make test-k8s-undockerized"

# Runs from host to allow full control over Kubernetes cluster
.PHONY: test-k8s-functional
test-k8s-functional:
	go test \
		-tags="test_kube,test_functional" \
 		-v \
 		-p 1 \
 		--timeout $(NUCLIO_GO_TEST_TIMEOUT) \
 		$(shell go list -tags="test_kube,test_functional" ./cmd/... ./pkg/...)


.PHONY: build-test
build-test: build-builder
	$(eval NUCLIO_TEST_KUBECTL_CLI_VERSION ?= v1.27.5)
	$(eval NUCLIO_TEST_KUBECTL_CLI_ARCH ?= $(if $(filter $(NUCLIO_ARCH),amd64),amd64,arm64))
	docker build \
        --build-arg GOARCH=$(NUCLIO_ARCH) \
		--build-arg NUCLIO_DOCKER_IMAGE_TAG=$(NUCLIO_DOCKER_IMAGE_TAG) \
		--build-arg DOCKER_CLI_ARCH=$(NUCLIO_DOCKER_CLIENT_ARCH) \
		--build-arg DOCKER_CLI_VERSION=$(NUCLIO_DOCKER_CLIENT_VERSION) \
		--build-arg NUCLIO_DOCKER_REPO=$(NUCLIO_DOCKER_REPO) \
		--build-arg KUBECTL_CLI_ARCH=$(NUCLIO_TEST_KUBECTL_CLI_ARCH) \
		--build-arg KUBECTL_CLI_VERSION=$(NUCLIO_TEST_KUBECTL_CLI_VERSION) \
		--file $(NUCLIO_DOCKER_TEST_DOCKERFILE_PATH) \
		--tag $(NUCLIO_DOCKER_TEST_TAG) .

#
# Test runtime wrappers
#

.PHONY: test-nodejs
test-nodejs:
	docker run \
	 --rm \
	 --volume $(NUCLIO_PATH)/pkg/processor/runtime/nodejs/js:/nuclio/nodejs \
	 --volume $(NUCLIO_PATH)/test:/nuclio/test \
	 --workdir /nuclio/nodejs \
	 --env RUN_MODE=CI \
	 node:16-alpine \
	 sh -c 'npm install && npm run lint && npm run test'

.PHONY: test-python
test-python:
	@set -e; \
	for runtime in 3.11 3.10 3.9 3.8 3.7; do \
		docker build \
			--build-arg PYTHON_IMAGE_TAG=$$runtime \
			--build-arg CACHEBUST=$(shell date +%s) \
			--file pkg/processor/runtime/python/test/Dockerfile \
			. ;\
	done

# list of tests which run for a long time, so it makes sense to run each of those in parallel
TEST_LIST_RUN_EACH_IN_PARALLEL = pkg/nuctl/test \
 								 pkg/processor/build/runtime/dotnetcore/test \
								 pkg/processor/build/runtime/golang/test  \
								 pkg/processor/build/runtime/java/test  \
								 pkg/processor/build/runtime/python/test  \
								 pkg/processor/runtime/python/test

.PHONY: list-all-dirs-with-tests
list-all-dirs-with-tests:
	@go list -f '{{ if .TestGoFiles }}{{.ImportPath }}{{ end }}' -tags="test_integration,test_local" ./cmd/... ./pkg/...  #| sed -r 's/github.com\/nuclio\/nuclio/./g'

.PHONY: list-tests-with-long-execution-time
list-tests-with-long-execution-time:
	@(for value in $(TEST_LIST_RUN_EACH_IN_PARALLEL); do pattern+="-e $$value "; done; make list-all-dirs-with-tests | grep $$pattern)

.PHONY: fast-tests
fast-tests:
	@for value in $(TEST_LIST_RUN_EACH_IN_PARALLEL); do pattern+="-v -e $$value "; done; make list-all-dirs-with-tests | grep $$pattern

.PHONY: nuctl-tests
nuctl-tests:
	@make list-all-dirs-with-tests | grep "pkg/nuctl/test"

.PHONY: dotnet-tests
dotnet-tests:
	@make list-all-dirs-with-tests | grep "pkg/processor/build/runtime/dotnetcore/test"

.PHONY: golang-tests
golang-tests:
	@make list-all-dirs-with-tests | grep "pkg/processor/build/runtime/golang/test"

.PHONY: java-tests
java-tests:
	@make list-all-dirs-with-tests | grep "pkg/processor/build/runtime/java/test"

.PHONY: python-tests
python-tests:
	@make list-all-dirs-with-tests | grep "pkg/processor/build/runtime/python/test"

.PHONY: python-runtime-tests
python-runtime-tests:
	@make list-all-dirs-with-tests | grep "pkg/processor/runtime/python/test"

#
# Go env
#

.PHONY: ensure-gopath
ensure-gopath:
ifndef GOPATH
	$(error GOPATH must be set)
endif

.PHONY: modules
modules: ensure-gopath
	@go mod download

.PHONY: targets
targets:
	@awk -F: '/^[^ \t="]+:/ && !/PHONY/ {print $$1}' Makefile | sort -u
