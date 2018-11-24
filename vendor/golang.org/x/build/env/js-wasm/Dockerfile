# Copyright 2018 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM {{REPO}}/linux-x86-sid:latest
MAINTAINER golang-dev <golang-dev@googlegroups.com>

ENV DEBIAN_FRONTEND noninteractive

# Add node.js to run the js/wasm tests.
RUN apt-get update && apt-get install -y \
	--no-install-recommends \
	nodejs \
	&& rm -rf /var/lib/apt/lists/*

CMD ["/usr/local/bin/stage0"]
