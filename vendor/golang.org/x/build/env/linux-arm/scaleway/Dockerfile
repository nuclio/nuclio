# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM scaleway/ubuntu:armhf-xenial

RUN apt-get update

RUN apt-get install --yes \
          gcc strace procps psmisc libc6-dev

RUN curl -L -o go1.8.1.tar.gz https://golang.org/dl/go1.8.1.linux-armv6l.tar.gz && \
    tar fxzv go1.8.1.tar.gz -C /usr/local

ENV GO_BOOTSTRAP=/usr/local/go

# compiled stage0 binary must be in working dir
COPY stage0 /usr/local/bin/stage0

ENV GO_BUILD_KEY_PATH /buildkey/gobuildkey
ENV GO_BUILD_KEY_DELETE_AFTER_READ true

# Not really, but we're in a container like Kubernetes, and this makes the syscall
# package happy:
ENV IN_KUBERNETES 1

ENV GO_BUILDER_ENV host-linux-arm-scaleway

# env specific
ARG buildlet_bucket

ENV META_BUILDLET_BINARY_URL "https://storage.googleapis.com/$buildlet_bucket/buildlet.linux-arm"

CMD ["/usr/local/bin/stage0"]