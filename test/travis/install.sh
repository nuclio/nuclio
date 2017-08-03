#!/bin/bash

# Fail on first error
set -e
# Print commands
set -x

outfile=${HOME}/.local/bin/kubectl

url_base=https://storage.googleapis.com/kubernetes-release/release
kube_ver=$(curl -s ${url_base}/stable.txt)
curl -L \
    --create-dirs -o ${outfile} \
    ${url_base}/${kube_ver}/bin/linux/amd64/kubectl
chmod +x ${outfile}
