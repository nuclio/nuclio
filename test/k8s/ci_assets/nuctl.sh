#!/usr/bin/env bash

set -e

if [[ -z "${NUCTL_BIN}" ]]; then
  NUCTL_BIN="$nuctl"
else
  NUCTL_BIN="${NUCTL_BIN}"
fi

if [[ -z "${NAMESPACE}" ]]; then
  NAMESPACE="default"
else
  NAMESPACE="${NAMESPACE}"
fi

if [[ -z "${FUNCTION_NAME}" ]]; then
  FUNCTION_NAME="test-function"
else
  FUNCTION_NAME="${FUNCTION_NAME}"
fi

if [[ -z "${NUCLIO_DASHBOARD_DEFAULT_ONBUILD_REGISTRY_URL}" ]]; then
  NUCLIO_DASHBOARD_DEFAULT_ONBUILD_REGISTRY_URL="localhost:500"
else
  NUCLIO_DASHBOARD_DEFAULT_ONBUILD_REGISTRY_URL="${REPO}"
fi

echo Deploying function ${FUNCTION_NAME}...

${NUCTL_BIN} \
  --verbose \
  deploy ${FUNCTION_NAME} \
  --path hack/examples/golang/helloworld/helloworld.go \
  --registry localhost:5000 \
  --run-registry localhost:5000 \
  --namespace ${NAMESPACE} \
  --no-pull

echo Invoking function ${FUNCTION_NAME}...

${NUCTL_BIN} \
  --verbose \
  invoke ${FUNCTION_NAME} \
  --namespace ${NAMESPACE}
