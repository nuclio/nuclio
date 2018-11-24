#!/bin/bash

# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -eu

ZONE=us-central1-f
INSTANCE_NAME="${1:-golang-buildlet}"

# Set, fetch credentials
echo ""
echo "Connecting to instance: "
echo ""

username="gopher"
password="gopher"
ip=$(gcloud compute instances describe "${INSTANCE_NAME}" --project="${PROJECT_ID}" --zone="${ZONE}" --format="value(networkInterfaces[0].networkIP)")

echo xfreerdp -u "${username}" -p "'${password}'" -n "${ip}" --ignore-certificate "${ip}"
xfreerdp -u "${username}" -p "${password}" -n "${ip}" --ignore-certificate "${ip}"
