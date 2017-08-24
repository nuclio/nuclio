#!/bin/bash
# Example install script. This will run on the processor container
# You need to provide path to this file in build.yaml

apt update
apt install -y --no-install-recommends \
    wget \
    zip 
