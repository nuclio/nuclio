# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
FROM debian:jessie
LABEL maintainer "golang-dev@googlegroups.com"

# openssh client is for the gomote ssh proxy client.
# telnet is for the gomote ssh proxy to windows. (no ssh server there)
# git-core, make, gcc, libc6-dev, and libx11-dev are to build 0intro/conterm,
# used to connect to plan9 instances.
RUN apt-get update && apt-get install -y \
	--no-install-recommends \
	ca-certificates \
	openssh-client \
	telnet \
	git-core make gcc libc6-dev libx11-dev \
	&& rm -rf /var/lib/apt/lists/*

# drawterm connects to plan9 instances like:
#    echo glenda123 | ./drawterm -a <addr> -c <addr> -u glenda -k user=glenda
# Where <addr> is the IP address of the Plan 9 instance on GCE,
# "glenda" is the username and "glenda123" is the password.
RUN git clone https://github.com/0intro/conterm /tmp/conterm && \
    cd /tmp/conterm && \
    CONF=unix make && mv /tmp/conterm/drawterm /usr/local/bin && \
    rm -rf /tmp/conterm

COPY coordinator /
ENTRYPOINT ["/coordinator"]
