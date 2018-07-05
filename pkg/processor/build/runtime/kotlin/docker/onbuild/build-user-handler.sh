#!/usr/bin/env sh

# Copyright 2018 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

# count how many jars there are in in /home/gradle/src/userHandler/src/main/kotlin (this is where the onbuild docker
# image puts the user provided files). if the user passed source, this should be 0. if the user
# passed a jar, this should be one. if it's neither, give up
num_jars=$(ls -1 /home/gradle/src/userHandler/src/main/kotlin/*.jar 2>/dev/null | wc -l)

# if there are zero jars, assume source and build
if [ $num_jars = "0" ]; then
    gradle tasks
    gradle userHandler

# if there is 1 jar, use it as the user handler. Move it to the place it would've been built
elif [ $num_jars = "1" ]; then

    # create the build dir
    mkdir -p /home/gradle/src/userHandler/build/libs

    # move the jar there
    cp $(ls -1 /home/gradle/src/userHandler/src/main/kotlin/*.jar) /home/gradle/src/userHandler/build/libs/user-handler.jar

# otherwise we have too many jars
else
    echo 'Found too many jars in directory'
    exit 1
fi
