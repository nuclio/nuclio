#!/bin/bash
# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Run "make update-deps" in each directory with such a target.

for x in $(git grep -l update-deps | grep Makefile); do (cd $(dirname $x) && make update-deps); done
