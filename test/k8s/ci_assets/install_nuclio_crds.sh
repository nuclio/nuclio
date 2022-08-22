# Copyright 2017 The Nuclio Authors.
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
#
#!/usr/bin/env bash

echo "Installing nuclio CRDs only"
helm install \
    --set controller.enabled=false \
    --set dashboard.enabled=false \
    --set autoscaler.enabled=false \
    --set dlx.enabled=false \
    --set rbac.create=false \
    --set crd.create=true \
    --debug \
    --wait \
    nuclio hack/k8s/helm/nuclio
