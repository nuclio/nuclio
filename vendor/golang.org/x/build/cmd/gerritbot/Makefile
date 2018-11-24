# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

MUTABLE_VERSION ?= latest
VERSION ?= $(shell git rev-parse --short HEAD)

IMAGE_STAGING := gcr.io/go-dashboard-dev/gerritbot
IMAGE_PROD := gcr.io/symbolic-datum-552/gerritbot

update-deps:
	go install golang.org/x/build/cmd/gitlock
	gitlock --update=Dockerfile golang.org/x/build/cmd/gerritbot

docker-prod: Dockerfile
	docker build --force-rm -f Dockerfile --tag=$(IMAGE_PROD):$(VERSION) ../..
	docker tag $(IMAGE_PROD):$(VERSION) $(IMAGE_PROD):$(MUTABLE_VERSION)
docker-staging: Dockerfile
	docker build --force-rm -f Dockerfile --tag=$(IMAGE_STAGING):$(VERSION) ../..
	docker tag $(IMAGE_STAGING):$(VERSION) $(IMAGE_STAGING):$(MUTABLE_VERSION)

push-prod: docker-prod
	docker push $(IMAGE_PROD):$(VERSION)
push-staging: docker-staging
	docker push $(IMAGE_STAGING):$(VERSION)

deploy-prod: push-prod
	go install golang.org/x/build/cmd/xb
	xb --prod kubectl set image deployment/gerritbot-deployment gerritbot=$(IMAGE_PROD):$(VERSION)
deploy-staging: push-staging
	go install golang.org/x/build/cmd/xb
	xb --staging kubectl set image deployment/gerritbot-deployment gerritbot=$(IMAGE_STAGING):$(VERSION)
