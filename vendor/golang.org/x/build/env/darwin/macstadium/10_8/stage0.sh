#!/bin/bash

# This is the file baked into the OS X 10.8 VM image.
#
# Normally our VMs & containers actually have the cmd/buildlet/stage0
# binary baked-in, but the Mac VM images are extra painful to
# configure, so there's an extra level of indirection in their boot
# process to give us this point of flexibility. This shell script then
# downloads run-builder-darwin-10_8.gz which is the cmd/buildlet/stage0
# binary, compressed.

set -e
url="https://storage.googleapis.com/go-builder-data/run-builder-darwin-10_8.gz"
while ! curl -f -o run-builder.gz "$url"; do
    echo
    echo "curl failed to fetch $url"
    echo "Sleeping before retrying..."
    sleep 2
done

set -x
gunzip -f run-builder.gz
chmod +x run-builder
exec ./run-builder
