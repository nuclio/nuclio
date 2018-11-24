#!/bin/bash
# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -e

gcloud compute instances delete vcs-test --zone=us-central1-a
gcloud compute instances create vcs-test --zone=us-central1-a \
	--address vcs-test \
	--image-project debian-cloud --image-family debian-9 \
	--machine-type n1-standard-1 \
	--service-account=vcs-test@symbolic-datum-552.iam.gserviceaccount.com \
	--scopes cloud-platform \
	--tags=allow-ssh,http-server,https-server

while sleep 5 && ! gcloud compute ssh vcs-test -- date; do
	echo 'waiting for machine to respond to ssh...'
done

gcloud compute ssh vcs-test -- sudo -n bash -c \''
	mkdir -p /home/vcweb/svn
	chown -R uucp:uucp /home/vcweb
	chmod -R 777 /home/vcweb
	apt-get update
	apt-get install -y mercurial fossil bzr git apache2 ed subversion libapache2-mod-svn
	perl -pie 's/80/8888/' /etc/apache2/ports.conf
	echo "
	    <Location /svn>
	      DAV svn
	      SVNParentPath /home/vcweb/svn
	      <LimitExcept GET PROPFIND OPTIONS REPORT>
	        Require all denied
	      </LimitExcept>
	    </Location>
	" >/etc/apache2/mods-enabled/dav_svn.conf
	apache2ctl restart
	systemctl enable apache2.service
'\'
