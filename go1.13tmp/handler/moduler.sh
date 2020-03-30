#!/usr/bin/env sh


# exit on failure
set -o errexit

# show command before execute
set -o xtrace

if [ ! -f "go.mod" ]; then
	mv /processor_go.mod go.mod
	mv /processor_go.sum go.sum
fi

# download missing modules & remove unused modules
go mod tidy

# Removing breadcrums
rm -rf /processor_go.mod /processor_go.sum
