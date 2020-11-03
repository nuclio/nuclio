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
