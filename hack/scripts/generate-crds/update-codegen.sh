#!/usr/bin/env bash

# Copyright 2023 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

SCRIPT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_ROOT}/../../.." && pwd)"

bash "${CODEGEN_PKG:-/code-generator}"/generate-groups.sh \
  "deepcopy,client,informer,lister" \
  github.com/nuclio/nuclio/pkg/platform/kube/client \
  github.com/nuclio/nuclio/pkg/platform/kube/apis \
  nuclio.io:v1beta1 \
  --output-base "${PROJECT_ROOT}" \
  --go-header-file "${SCRIPT_ROOT}"/boilerplate.go.txt

# merge outputs with current source code
rsync --recursive \
 --remove-source-files \
 --ignore-times \
 "${PROJECT_ROOT}/github.com/nuclio/nuclio/pkg/" /output

# delete generated code
rm -rf "${PROJECT_ROOT}/github.com"
