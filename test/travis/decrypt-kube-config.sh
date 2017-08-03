#!/bin/bash
# Decrypt kubernetes configuration file for test cluster
# ENCRYPTION_KEY is provided by travis

cfg_file=~/.kube/config

set -e

if [ -f ${cfg_file} ]; then
    backup=${cfg_file}.$(date +%Y-%m-%d:%H:%M:%S)
    cp -v ${cfg_file} ${backup}
fi

mkdir -p ~/.kube
rm -f ${cfg_file}
openssl aes-256-cbc \
    -k "${ENCRYPTION_KEY}" \
    -in ./test/travis/kube_config.enc \
    -out ${cfg_file} \
    -d
