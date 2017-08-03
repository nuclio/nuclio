#!/bin/bash

# Fail on first error
set -e
# Print commands
set -x

url_base=https://storage.googleapis.com/kubernetes-release/release
kube_ver=$(curl -s ${url_base}/stable.txt)
curl -L -u /usr/local/bin/kubectl ${url_base}/${kube_ver}/bin/linux/amd64/kubectl
chmod +x /usr/local/bin/kubectl
