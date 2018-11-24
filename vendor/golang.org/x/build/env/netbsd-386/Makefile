# Copyright 2018 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

upload-staging:
	go install golang.org/x/build/cmd/upload
	upload --verbose --file=netbsd-i386-8.0_2018Q1.tar.gz --public dev-go-builder-data/vm-image-netbsd-i386-8.0_2018Q1.tar.gz

upload-prod:
	go install golang.org/x/build/cmd/upload
	upload --verbose --file=netbsd-i386-8.0_2018Q1.tar.gz --public go-builder-data/vm-image-netbsd-i386-8.0_2018Q1.tar.gz

create-image-staging:
	gcloud compute --project go-dashboard-dev images create netbsd-i386-8-0-2018q1 --source-uri gs://dev-go-builder-data/vm-image-netbsd-i386-8.0_2018Q1.tar.gz

create-image-prod:
	gcloud compute --project symbolic-datum-552 images create netbsd-i386-8-0-2018q1 --source-uri gs://go-builder-data/vm-image-netbsd-i386-8.0_2018Q1.tar.gz

clean:
	rm -rf netbsd-*.tar.gz work-NetBSD-* anita-*.tar.gz anita-*/
