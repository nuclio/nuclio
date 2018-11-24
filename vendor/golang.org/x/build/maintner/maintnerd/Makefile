# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

MUTABLE_VERSION ?= latest
VERSION ?= $(shell git rev-parse --short HEAD)

IMAGE_STAGING := gcr.io/go-dashboard-dev/maintnerd
IMAGE_PROD := gcr.io/symbolic-datum-552/maintnerd

DOCKER_IMAGE_build0=build0/maintnerd:latest
DOCKER_CTR_build0=maintnerd-build0

build0: *.go Dockerfile.0
	docker build --force-rm -f Dockerfile.0 --tag=$(DOCKER_IMAGE_build0) ../..

maintnerd: build0
	docker create --name $(DOCKER_CTR_build0) $(DOCKER_IMAGE_build0)
	docker cp $(DOCKER_CTR_build0):/go/bin/$@ $@
	docker rm $(DOCKER_CTR_build0)

ca-certificates.crt:
	docker create --name $(DOCKER_CTR_build0) $(DOCKER_IMAGE_build0)
	docker cp $(DOCKER_CTR_build0):/etc/ssl/certs/$@ $@
	docker rm $(DOCKER_CTR_build0)

update-deps:
	go install golang.org/x/build/cmd/gitlock
	gitlock --update=Dockerfile.0 golang.org/x/build/maintner/maintnerd

docker-prod: Dockerfile maintnerd ca-certificates.crt
	docker build --force-rm --tag=$(IMAGE_PROD):$(VERSION) .
	docker tag $(IMAGE_PROD):$(VERSION) $(IMAGE_PROD):$(MUTABLE_VERSION)
docker-staging: Dockerfile maintnerd ca-certificates.crt
	docker build --force-rm --tag=$(IMAGE_STAGING):$(VERSION) .
	docker tag $(IMAGE_STAGING):$(VERSION) $(IMAGE_STAGING):$(MUTABLE_VERSION)

push-prod: docker-prod
	docker push $(IMAGE_PROD):$(MUTABLE_VERSION)
	docker push $(IMAGE_PROD):$(VERSION)
push-staging: docker-staging
	docker push $(IMAGE_STAGING):$(MUTABLE_VERSION)
	docker push $(IMAGE_STAGING):$(VERSION)

deploy-prod: push-prod
	go install golang.org/x/build/cmd/xb
	xb --prod kubectl set image deployment/maintnerd-deployment maintnerd=$(IMAGE_PROD):$(VERSION)
deploy-staging: push-staging
	go install golang.org/x/build/cmd/xb
	xb --staging kubectl set image deployment/maintnerd-deployment maintnerd=$(IMAGE_STAGING):$(VERSION)

.PHONY: clean
clean:
	$(RM) maintnerd
	$(RM) ca-certificates.crt
