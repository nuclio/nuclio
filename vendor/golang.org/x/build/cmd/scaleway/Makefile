# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

VERSION=latest
DOCKER_IMAGE_build0=build0/scaleway:latest
DOCKER_CTR_build0=scaleway-build0

build0: *.go Dockerfile.0
	docker build --force-rm -f Dockerfile.0 --tag=$(DOCKER_IMAGE_build0) ../..

scaleway: build0
	docker create --name $(DOCKER_CTR_build0) $(DOCKER_IMAGE_build0)
	docker cp $(DOCKER_CTR_build0):/go/bin/$@ $@
	docker rm $(DOCKER_CTR_build0)

ca-certificates.crt:
	docker create --name $(DOCKER_CTR_build0) $(DOCKER_IMAGE_build0)
	docker cp $(DOCKER_CTR_build0):/etc/ssl/certs/$@ $@
	docker rm $(DOCKER_CTR_build0)

update-deps:
	go install golang.org/x/build/cmd/gitlock
	gitlock --update=Dockerfile.0 golang.org/x/build/cmd/scaleway

docker-prod: Dockerfile scaleway ca-certificates.crt
	docker build --force-rm --tag=gcr.io/symbolic-datum-552/scaleway:$(VERSION) .
docker-dev: Dockerfile scaleway ca-certificates.crt
	docker build --force-rm --tag=gcr.io/go-dashboard-dev/scaleway:latest .

push-prod: docker-prod
	docker push gcr.io/symbolic-datum-552/scaleway:$(VERSION)
push-dev: docker-dev
	docker push gcr.io/go-dashboard-dev/scaleway:latest

.PHONY: clean
clean:
	$(RM) scaleway
	$(RM) ca-certificates.crt
