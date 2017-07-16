#!/usr/bin/env bash

if [[ -e .deps ]]; then
    apt-get update
    for dep in $(cat .deps); do
        apt-get install -y --no-install-recommends $dep
    done
    rm -rf /var/lib/apt/lists/*
fi

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go get -a -installsuffix cgo github.com/nuclio/nuclio/cmd/processor
