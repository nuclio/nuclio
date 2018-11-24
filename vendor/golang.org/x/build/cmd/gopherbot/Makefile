# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

MUTABLE_VERSION ?= latest
VERSION ?= $(shell git rev-parse --short HEAD)

IMAGE_STAGING := gcr.io/go-dashboard-dev/gopherbot
IMAGE_PROD := gcr.io/symbolic-datum-552/gopherbot

update-deps:
	go install golang.org/x/build/cmd/gitlock
	gitlock --update=Dockerfile golang.org/x/build/cmd/gopherbot

docker-image: Dockerfile *.go
	docker build --force-rm -f Dockerfile --tag=$(IMAGE_PROD):$(VERSION) ../..
	docker tag $(IMAGE_PROD):$(VERSION) $(IMAGE_PROD):$(MUTABLE_VERSION)
	docker tag $(IMAGE_PROD):$(VERSION) $(IMAGE_STAGING):$(VERSION)
	docker tag $(IMAGE_PROD):$(VERSION) $(IMAGE_STAGING):$(MUTABLE_VERSION)

push-prod: docker-image
	docker push $(IMAGE_PROD):$(MUTABLE_VERSION)
	docker push $(IMAGE_PROD):$(VERSION)
push-staging: docker-image
	docker push $(IMAGE_STAGING):$(MUTABLE_VERSION)
	docker push $(IMAGE_STAGING):$(VERSION)

deploy-prod: push-prod
	go install golang.org/x/build/cmd/xb
	xb --prod kubectl set image deployment/gopherbot-deployment gopherbot=$(IMAGE_PROD):$(VERSION)
deploy-staging: push-staging
	go install golang.org/x/build/cmd/xb
	xb --staging kubectl set image deployment/gopherbot-deployment gopherbot=$(IMAGE_STAGING):$(VERSION)
