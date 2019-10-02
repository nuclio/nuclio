#!/bin/bash
. support/scripts/functions.sh

# Run only the unit tests and not integration tests
go test -cover -race $(local_go_pkgs)
