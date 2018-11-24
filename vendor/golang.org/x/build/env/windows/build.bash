#!/bin/bash

# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

#
# Creates a new windows VM running the buildlet
#

# SPINNING UP A TEST INSTANCE
# Create a windows instance and bakes the buildlet into the startup.
#
# PROJECT_ID=[your GCP project] BASE_IMAGE=windows-server-2012-r2-dc-v20171010 IMAGE_PROJECT=windows-cloud ./build.bash
# PROJECT_ID=[your GCP project] ./rdp.bash

# CREATED AN IMAGE FOR BUILDER DASHBOARD
# Creates an image and validates it for use in the dashboard.
#
# Steps:
#  - Create new VM
#  - Run startup.ps1, restart
#  - Wait till buildlet process is up
#  - Stop VM
#  - Capture image
#  - Create a new VM from the image
#  - Wait till buildlet process is up
#  - Run ./test_buildlet.bash
#
# PROJECT_ID=[your GCP project] BASE_IMAGE=windows-server-2012-r2-dc-v20171010 IMAGE_PROJECT=windows-cloud CAPTURE_IMAGE=true ./build.bash

set -eu

ZONE="us-central1-f"
BUILDER_PREFIX="${1-golang}"
IMAGE_NAME="${1-${BASE_IMAGE}}"
INSTANCE_NAME="${BUILDER_PREFIX}-buildlet"
TEST_INSTANCE_NAME="${BUILDER_PREFIX}-buildlet-test"
MACHINE_TYPE="n1-standard-4"
BUILDLET_IMAGE="windows-amd64-${IMAGE_NAME}"
IMAGE_PROJECT=$IMAGE_PROJECT
BASE_IMAGE=$BASE_IMAGE
CAPTURE_IMAGE="${CAPTURE_IMAGE-false}"

function wait_for_buildlet() {
  ip=$1
  seconds=5

  echo "Waiting for buildlet at ${ip} to become responsive"
  until curl "http://${ip}" 2>/dev/null; do
    echo "retrying ${ip} in ${seconds} seconds"
    sleep "${seconds}"
  done
}

#
# 0. Cleanup images/instances from prior runs
#
echo "Destroying existing instances (if exists)"
yes "Y" | gcloud compute instances delete "$INSTANCE_NAME" --project="$PROJECT_ID" --zone="$ZONE" || true
yes "Y" | gcloud compute instances delete "$TEST_INSTANCE_NAME" --project="$PROJECT_ID" --zone="$ZONE" || true
echo "Destroying existing image (if exists)"
yes "Y" | gcloud compute images delete "$BUILDLET_IMAGE" --project="$PROJECT_ID" || true


#
# 1. Create base instance
#
echo "Creating target instance"
gcloud compute instances create --machine-type="$MACHINE_TYPE" "$INSTANCE_NAME" \
        --image "$BASE_IMAGE" --image-project "$IMAGE_PROJECT" \
        --project="$PROJECT_ID" --zone="$ZONE" \
        --metadata="buildlet-binary-url=https://storage.googleapis.com/go-builder-data/buildlet.windows-amd64" \
        --metadata-from-file=windows-startup-script-ps1=startup.ps1

echo ""
echo "Follow logs with:"
echo ""
echo gcloud compute instances tail-serial-port-output "$INSTANCE_NAME" --zone="$ZONE" --project="$PROJECT_ID"
echo ""
ip=$(gcloud compute instances describe "$INSTANCE_NAME" --project="$PROJECT_ID" --zone="$ZONE" --format="value(networkInterfaces[0].networkIP)")

wait_for_buildlet "$ip"

if [ ! "$CAPTURE_IMAGE" = "true" ]; then
  exit 0
fi

#
# 2. Image base instance
#

echo "Shutting down instance"
gcloud compute instances stop "$INSTANCE_NAME" \
        --project="$PROJECT_ID" --zone="$ZONE"

echo "Capturing image"
gcloud compute images create "$BUILDLET_IMAGE" --source-disk "$INSTANCE_NAME" --source-disk-zone "$ZONE" --project="$PROJECT_ID"

echo "Removing base machine"
yes "Y" | gcloud compute instances delete "$INSTANCE_NAME" --project="$PROJECT_ID" --zone="$ZONE" || true

#
# 3. Verify image is valid
#

echo "Creating new machine with image"
gcloud compute instances create --machine-type="$MACHINE_TYPE" --image "$BUILDLET_IMAGE" "$TEST_INSTANCE_NAME" \
       --project="$PROJECT_ID" --metadata="buildlet-binary-url=https://storage.googleapis.com/go-builder-data/buildlet.windows-amd64" \
       --zone="$ZONE"

test_image_ip=$(gcloud compute instances describe "$TEST_INSTANCE_NAME" --project="$PROJECT_ID" --zone="$ZONE" --format="value(networkInterfaces[0].networkIP)")
wait_for_buildlet "$test_image_ip"

echo "Performing test build"
./test_buildlet.bash "$test_image_ip"

echo "Removing test instance"
yes "Y" | gcloud compute instances delete "$TEST_INSTANCE_NAME" --project="$PROJECT_ID" --zone="$ZONE" || true

echo "Success! A new buildlet can be created with the following command"
echo "gcloud compute instances create --machine-type='$MACHINE_TYPE' '$INSTANCE_NAME' \
--metadata='buildlet-binary-url=https://storage.googleapis.com/go-builder-data/buildlet.windows-amd64' \
--image '$BUILDLET_IMAGE' --image-project '$PROJECT_ID'"
