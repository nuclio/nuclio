#!/bin/bash

# Run only the integration tests
# go test -race ./test
echo "Warning: Cannot find a good way to inject AWS credential to hmake container"
echo "Don't use hmake ci. Use the following command directly"
echo "go test -race ./test"
