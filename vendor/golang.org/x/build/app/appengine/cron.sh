#!/bin/sh

# Copyright 2016 The Go Authors.  All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# AppEngine has a single Cron configuration for all services and
# versions in the project. To load the cron configuration, this script
# concatenates the configuration from the two apps that use cron.

set -x -e

cd "$(dirname "$0")"

[ -e cron.yaml ] && (echo "cron.yaml already exists!"; exit 1)
(echo "cron:"; cat cron-part.yaml ../devapp/cron-part.yaml) > cron.yaml
appcfg.py update_cron .
rm cron.yaml
