#!/bin/bash
# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -e

info="$USER $(date)"
GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build "-ldflags=\"-X=main.buildInfo=$info\"" -o vcweb.exe ./vcweb
trap "rm -f vcweb.exe" EXIT

gcloud beta compute scp vcweb.exe vcs-test:

gcloud compute ssh vcs-test -- sudo -n bash -c \''
	mv vcweb.exe /usr/bin/vcweb
	chmod a+rx /usr/bin/vcweb
	systemctl restart vcweb.service
	systemctl status -l vcweb.service
'\'
