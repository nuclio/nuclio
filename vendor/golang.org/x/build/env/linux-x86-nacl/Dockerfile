# Copyright 2015 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# gobuilders/linux-x86-nacl for 32- and 64-bit nacl.
#
# We need more modern libc than Debian stable as used in base, so we're
# using Debian sid instead.

FROM {{REPO}}/linux-x86-sid:latest
MAINTAINER golang-dev <golang-dev@googlegroups.com>

ENV DEBIAN_FRONTEND noninteractive

RUN apt-get update && apt-get install -y \
	--no-install-recommends \
	bzip2 \
	lib32stdc++6 \
	&& ( \
		cd /usr/bin \
		&& curl -s -O https://storage.googleapis.com/nativeclient-mirror/nacl/nacl_sdk/trunk.544461/naclsdk_linux.tar.bz2 \
		&& tar xjf naclsdk_linux.tar.bz2 --strip-components=2 pepper_67/tools/sel_ldr_x86_32 pepper_67/tools/sel_ldr_x86_64 \
		&& rm naclsdk_linux.tar.bz2 \
	) \
	&& apt-get purge -y bzip2 \
	&& rm -rf /var/lib/apt/lists/*

COPY go_nacl_386_exec /usr/bin/
COPY go_nacl_amd64p32_exec /usr/bin/

CMD ["/usr/local/bin/stage0"]
