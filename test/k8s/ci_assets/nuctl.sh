#!/usr/bin/env bash

set -e

NUCTL_BIN="${NUCTL_BIN:-nuctl}"
NAMESPACE="${NAMESPACE:-default}"

FUNCTION_NAME="test-function"

echo "##############################################"
echo "Cleaning up function ${FUNCTION_NAME}..."
echo "##############################################"
echo

${NUCTL_BIN} --verbose delete function ${FUNCTION_NAME} || true

echo "##############################################"
echo "Deploying function ${FUNCTION_NAME}..."
echo "##############################################"
echo

${NUCTL_BIN} \
  --verbose \
  deploy ${FUNCTION_NAME} \
  --path=hack/examples/golang/helloworld/helloworld.go \
  --registry=localhost:5000 \
  --namespace=${NAMESPACE} \
  --no-pull

echo "##############################################"
echo "Invoking function ${FUNCTION_NAME}..."
echo "##############################################"
echo

${NUCTL_BIN} \
  --verbose \
  invoke ${FUNCTION_NAME} \
  --namespace=${NAMESPACE}

FUNCTION_NAME="s3-fast-failure"

echo "##############################################"
echo "Cleaning up function ${FUNCTION_NAME}..."
echo "##############################################"
echo

${NUCTL_BIN} --verbose delete function ${FUNCTION_NAME} || true

echo "##############################################"
echo "Deploying function ${FUNCTION_NAME}..."
echo "##############################################"
echo

if [[ $(${NUCTL_BIN} \
  --verbose \
  deploy ${FUNCTION_NAME} \
  --code-entry-type=s3 \
  --file=test/_function_configs/error/s3_codeentry/function.yaml \
  --registry=localhost:5000 \
  --namespace=${NAMESPACE} \
  --no-pull | tee /dev/tty | grep "Failed to download file from s3") ]]; then
  echo
  echo "SUCCESS: Function ${FUNCTION_NAME} deployment failed expectedly"
else
  echo
  echo "FAILURE: Function ${FUNCTION_NAME} deployment passed or failed unexpectedly"
  exit 1
fi

echo "##############################################"
echo "nuctl test Done"
echo "##############################################"
