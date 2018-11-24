#!/bin/bash
# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -e

gcloud beta compute scp vcweb.service vcweb-*.socket vcs-test:

gcloud compute ssh vcs-test -- sudo -n bash -c \''
	systemctl stop vcweb.service
	systemctl disable vcweb.service
	rm -f /lib/systemd/system/vcweb* /etc/systemd/system/*/vcweb*
	
	mv vcweb.exe /usr/bin/vcweb
	mv vcweb.service vcweb-*.socket /lib/systemd/system
	systemctl enable vcweb.service
'\'
