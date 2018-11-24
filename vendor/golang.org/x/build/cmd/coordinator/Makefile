# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

usage: FORCE
	# See Makefile targets.
	exit 1

FORCE:

# After "make upload", either reboot the machine, or ssh to it and:
#   sudo systemctl restart gobuild.service
# And watch its logs with:
#   sudo journalctl -f -u gobuild.service
upload: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=linux-amd64 --file=go:golang.org/x/build/cmd/coordinator --public go-builder-data/coordinator

dev-upload: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=linux-amd64 --file=go:golang.org/x/build/cmd/coordinator --public dev-go-builder-data/coordinator

update-deps:
	go install golang.org/x/build/cmd/gitlock
	gitlock --update=Dockerfile.0 golang.org/x/build/cmd/coordinator

MUTABLE_VERSION ?= latest
VERSION ?= $(shell git rev-parse --short HEAD)

IMAGE_STAGING := gcr.io/go-dashboard-dev/coordinator
IMAGE_PROD := gcr.io/symbolic-datum-552/coordinator

DOCKER_IMAGE_build0=build0/coordinator:latest
DOCKER_CTR_build0=coordinator-build0

build0: *.go Dockerfile.0
	docker build --force-rm -f Dockerfile.0 --build-arg "version=$$(./version.sh)" --tag=$(DOCKER_IMAGE_build0) ../..

coordinator: build0
	docker create --name $(DOCKER_CTR_build0) $(DOCKER_IMAGE_build0)
	docker cp $(DOCKER_CTR_build0):/go/bin/$@ $@
	docker rm $(DOCKER_CTR_build0)

docker-prod: Dockerfile coordinator
	docker build --force-rm --tag=$(IMAGE_PROD):$(VERSION) .
	docker tag $(IMAGE_PROD):$(VERSION) $(IMAGE_PROD):$(MUTABLE_VERSION)
docker-staging: Dockerfile coordinator
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
	xb --prod kubectl set image deployment/coordinator-deployment coordinator=$(IMAGE_PROD):$(VERSION)
deploy-staging: push-staging
	go install golang.org/x/build/cmd/xb
	xb --staging kubectl set image deployment/coordinator-deployment coordinator=$(IMAGE_STAGING):$(VERSION)

clean: FORCE
	$(RM) coordinator
	$(RM) ca-certificates.crt
