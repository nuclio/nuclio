#!/bin/bash

# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -e -u

declare -A public_images

public_images=(
         ['server-2016-v7']='windows-server-2016-dc-core-v20180612'
         ['server-2008r2-v7']='windows-server-2008-r2-dc-v20180612'
         ['server-2012r2-v7']='windows-server-2012-r2-dc-core-v20180612'
       )

mkdir -p out

for image in "${!public_images[@]}"; do
    prefix=$image
    base_image=${public_images[$image]}

    CAPTURE_IMAGE=true BASE_IMAGE="$base_image" IMAGE_PROJECT='windows-cloud' ./build.bash "$prefix" |& tee "out/${base_image}.txt" &
done


wait
