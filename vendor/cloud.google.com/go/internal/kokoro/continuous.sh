#!/bin/bash

export GOOGLE_APPLICATION_CREDENTIALS=$KOKORO_KEYSTORE_DIR/72523_go_integration_service_account
export GCLOUD_TESTS_GOLANG_PROJECT_ID=dulcet-port-762
export GCLOUD_TESTS_GOLANG_KEY=$GOOGLE_APPLICATION_CREDENTIALS
export GCLOUD_TESTS_GOLANG_FIRESTORE_PROJECT_ID=gcloud-golang-firestore-tests
export GCLOUD_TESTS_GOLANG_FIRESTORE_KEY=$KOKORO_KEYSTORE_DIR/72523_go_firestore_integration_service_account
export GCLOUD_TESTS_API_KEY=`cat $KOKORO_KEYSTORE_DIR/72523_go_gcloud_tests_api_key`
export GCLOUD_TESTS_GOLANG_KEYRING=projects/dulcet-port-762/locations/global/keyRings/go-integration-test

# Fail on any error
set -eo pipefail

# Display commands being run
set -x

# cd to project dir on Kokoro instance
cd git/gocloud

go version

# Set $GOPATH
export GOPATH="$HOME/go"
export GOCLOUD_HOME=$GOPATH/src/cloud.google.com/go/
export PATH="$GOPATH/bin:$PATH"
mkdir -p $GOCLOUD_HOME

# Move code into $GOPATH and get dependencies
git clone . $GOCLOUD_HOME
cd $GOCLOUD_HOME

try3() { eval "$*" || eval "$*" || eval "$*"; }
try3 go get -v -t ./...

./internal/kokoro/vet.sh

# Run tests and tee output to log file, to be pushed to GCS as artifact.
go test -race -v -timeout 15m ./... 2>&1 | tee $KOKORO_ARTIFACTS_DIR/$KOKORO_GERRIT_CHANGE_NUMBER.txt