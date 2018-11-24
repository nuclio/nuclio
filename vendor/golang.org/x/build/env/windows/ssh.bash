#!/bin/bash

# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -eu

ZONE=us-central1-f
INSTANCE_NAME="${1:-golang-buildlet}"

username="gopher"
password="gopher"
ip=$(gcloud compute instances describe "${INSTANCE_NAME}" --project="${PROJECT_ID}" --zone="${ZONE}" --format="value(networkInterfaces[0].networkIP)")

echo ""
echo "Connecting to instance ${INSTANCE_NAME}"
echo ""
echo "When prompted for password, use: ${password}"
echo ""
ssh -o PreferredAuthentications=password -o PubkeyAuthentication=no -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no gopher@${ip}
