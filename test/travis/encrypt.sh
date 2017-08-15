#!/bin/bash
# Encrypt files to be used in travis
# Ask around for password :) On travis it'll be avaiable as ENCRYPTION_KEY
# environment variable

case $1 in
    -h | --help ) echo "usage: $(basename $0) PASSWD INPUT OUTPUT"; exit;;
esac

if [ $# -ne 3 ]; then
    echo "error: wrong number of arguments"
    exit
fi

openssl aes-256-cbc -k ${1} -in ${2} -out ${3}
