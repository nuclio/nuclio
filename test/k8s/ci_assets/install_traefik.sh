#!/usr/bin/env bash
# Copyright 2026 The Nuclio Authors.
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

# install Gateway API CRDs
kubectl apply --server-side -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.5.1/standard-install.yaml

# install traefik
helm repo add traefik https://traefik.github.io/charts
helm repo update

helm install traefik traefik/traefik \
  --namespace traefik --create-namespace \
  --set ports.web.hostPort=80 \
  --set ports.websecure.hostPort=443 \
  --set deployment.kind=DaemonSet \
  --set service.type=ClusterIP \
  --set providers.kubernetesIngressNGINX.enabled=true \
  --set providers.kubernetesGateway.enabled=true \
  --set gateway.enabled=false \
  --set gatewayClass.enabled=true \
  --version 40.0.0-ea.3

# create nginx IngressClass standalone resource mapping to Traefik
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: nginx
spec:
  controller: k8s.io/ingress-nginx
EOF