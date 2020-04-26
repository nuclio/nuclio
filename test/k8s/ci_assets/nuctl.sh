#!/usr/bin/env bash

set -e

NUCTL_BIN="${NUCTL_BIN:-nuctl}"
NAMESPACE="${NAMESPACE:-default}"
FUNCTION_NAME="${FUNCTION_NAME:-test-function}"

echo "##############################################"
echo "Deploying function ${FUNCTION_NAME}..."
echo "##############################################"
echo

${NUCTL_BIN} \
  --verbose \
  deploy ${FUNCTION_NAME} \
  --path hack/examples/golang/helloworld/helloworld.go \
  --registry localhost:5000 \
  --run-registry localhost:5000 \
  --namespace ${NAMESPACE} \
  --no-pull

echo "##############################################"
echo "Invoking function ${FUNCTION_NAME}..."
echo "##############################################"
echo

${NUCTL_BIN} \
  --verbose \
  invoke ${FUNCTION_NAME} \
  --namespace ${NAMESPACE}


echo "##############################################"
echo "nuctl test Done"
echo "##############################################"
