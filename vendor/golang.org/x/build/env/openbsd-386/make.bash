#!/bin/bash
# Copyright 2014 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -e
set -u

ARCH=i386 ../openbsd-amd64/make.bash
