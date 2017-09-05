#!/bin/bash

if [[ -e .deps ]]; then
    apt-get update
    for dep in $(cat .deps); do
        apt-get install -y --no-install-recommends $dep
    done
    rm -rf /var/lib/apt/lists/*
fi
