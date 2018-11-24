#!/bin/sh

# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

VERSION=$(git rev-parse HEAD)
if ! git diff-index HEAD --quiet || ! git diff-files --quiet; then
  VERSION=$VERSION-dirty
  dirty=1
fi
if [ -n "$dirty" ] || [ -n "$(git rev-list '@{upstream}..HEAD')" ]; then
  VERSION=$VERSION-$USER-$(date -u +%Y-%m-%dT%H:%M:%SZ)
fi
echo "$VERSION"
