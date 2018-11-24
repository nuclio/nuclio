# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Compiled stage0 binary must be in working dir.

STAGING_BUCKET=dev-go-builder-data
PROD_BUCKET=go-builder-data

staging: Dockerfile
	docker build --build-arg buildlet_bucket=$(STAGING_BUCKET) -t gobuilder-arm-scaleway:latest .

prod: Dockerfile
	docker build --build-arg buildlet_bucket=$(PROD_BUCKET) -t gobuilder-arm-scaleway:latest .
