# Copyright 2018 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

IMAGE_NAME=$(shell basename $(CURDIR))

usage:
	echo "see Makefile for usage for building $(IMAGE_NAME)"

docker: Dockerfile
	go install golang.org/x/build/cmd/xb
	xb docker build -t golang/$(IMAGE_NAME) .

push-staging: docker
	go install golang.org/x/build/cmd/xb
	xb --staging docker tag golang/$(IMAGE_NAME) REPO/$(IMAGE_NAME):latest
	xb --staging docker push REPO/$(IMAGE_NAME):latest

push-prod: docker
	go install golang.org/x/build/cmd/xb
	xb --prod docker tag golang/$(IMAGE_NAME) REPO/$(IMAGE_NAME):latest
	xb --prod docker push REPO/$(IMAGE_NAME):latest
