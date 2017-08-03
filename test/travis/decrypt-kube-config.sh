#!/bin/bash
# Decrypt kubernetes configuration file for test cluster
# ENCRYPTION_KEY is provided by travis

cfg_file=~/.kube/config

set -e

rm -f ${cfg_file}
openssl aes-256-cbc \
    -k "${ENCRYPTION_KEY}" \
    -in ./test/travis/kube_config.enc \
    -out ${cfg_file} \
    -d
