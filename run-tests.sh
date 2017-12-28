#!/bin/bash
# Run tests, send output to log

# Fail on pipe since we use tee
set -o pipefail
# Exit on first error
set -e

log_file=/tmp/nuclio-test-$(date +%Y-%m-%dT%H:%M:%S).log
trap "echo; echo Log file at ${log_file}" EXIT
2>&1 make test | tee ${log_file}
