#!/bin/bash

# This is the file baked into the OS X 10.11 VM image. It is ALSO
# baked into the macOS 10.12 Sierra image. (That is, both 10.11 and
# 10.12 download and run the run-builder-darwin-10_11.gz URL)
#
# Normally our VMs & containers actually have the cmd/buildlet/stage0
# binary baked-in, but the Mac VM images are extra painful to
# configure, so there's an extra level of indirection in their boot
# process to give us this point of flexibility. This shell script then
# downloads run-builder-darwin-10_11.gz which is the cmd/buildlet/stage0
# binary, compressed.

set -e
url="https://storage.googleapis.com/go-builder-data/run-builder-darwin-10_11.gz"
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
