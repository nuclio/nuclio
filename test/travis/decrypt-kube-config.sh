#!/bin/bash
# Decrypt kubernetes configuration file for test cluster
# ENCRYPTION_KEY is provided by travis

cfg_file=~/.kube/config
enc_file=./test/travis/kube_config.enc

set -e
set -x

if [ -f ${cfg_file} ]; then
    backup=${cfg_file}.$(date +%Y-%m-%d:%H:%M:%S)
    mv -v ${cfg_file} ${backup}
fi

mkdir -p ~/.kube
echo "${enc_file} -> ${cfg_file}"
openssl aes-256-cbc -k "${ENCRYPTION_KEY}" -in ${enc_file} -out ${cfg_file} -d
