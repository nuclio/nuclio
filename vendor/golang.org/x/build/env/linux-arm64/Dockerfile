# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM mickaelguene/arm64-debian:jessie

ENV DEBIAN_FRONTEND noninteractive

RUN apt-get update && \
    apt-get install --yes \
          curl gcc strace ca-certificates \
          procps lsof psmisc
RUN apt-get install --yes --no-install-recommends openssh-server

RUN mkdir /usr/local/go-bootstrap && \
    curl --silent https://storage.googleapis.com/go-builder-data/gobootstrap-linux-arm64.tar.gz | \
    tar -C /usr/local/go-bootstrap -zxv

ENV GOROOT_BOOTSTRAP /usr/local/go-bootstrap
RUN curl -o  /usr/local/bin/stage0 https://storage.googleapis.com/go-builder-data/buildlet-stage0.linux-arm64 && \
    chmod +x /usr/local/bin/stage0

ENV GO_BUILD_KEY_DELETE_AFTER_READ true
ENV GO_BUILD_KEY_PATH /buildkey/gobuildkey

# Not really, but we're in a container like Kubernetes, and this makes the syscall
# package happy:
ENV IN_KUBERNETES 1


