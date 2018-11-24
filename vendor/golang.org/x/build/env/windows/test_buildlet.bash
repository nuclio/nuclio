#!/bin/bash

# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -ue

hostname="$1"
BUILDLET="windows-amd64-2012@${hostname}"

echo "Pushing go1.4, go1.9beta2 to buildlet"
gomote puttar -url https://storage.googleapis.com/golang/go1.9beta2.src.tar.gz "$BUILDLET"
gomote put14 "$BUILDLET"

echo "Building go (32-bit)"
gomote run -e GOARCH=386 -e GOHOSTARCH=386 -path 'C:/godep/gcc32/bin,$WORKDIR/go/bin,$PATH' -e 'GOROOT=c:\workdir\go' "$BUILDLET" go/src/make.bat

# Go1.8 has failing tests on some windows versions.
# Push a new release when avaliable or update this to use master.
echo "Running tests for go (32-bit)"
#gomote run -e GOARCH=386 -e GOHOSTARCH=386 -path 'C:/godep/gcc32/bin,$WORKDIR/go/bin,$PATH' -e 'GOROOT=C:\workdir\go' "$BUILDLET" go/bin/go.exe tool dist test -v --no-rebuild

echo "Building go (64-bit)"
gomote run -path '$PATH,C:/godep/gcc64/bin,$WORKDIR/go/bin,$PATH' -e 'GOROOT=c:\workdir\go' "$BUILDLET" go/src/make.bat

echo "Running tests for go (64-bit)"
#gomote run -path 'C:/godep/gcc64/bin,$WORKDIR/go/bin,$PATH' -e 'GOROOT=C:\workdir\go' "$BUILDLET" go/bin/go.exe tool dist test -v --no-rebuild
