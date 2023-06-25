#!/bin/sh

# Copyright 2023 The Nuclio Authors.
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

EVENTBODY=$(cat)

sleep_and_exit() {
  sleepTimeout=$(echo ${EVENTBODY} | cut -d' ' -f2)
	sleep ${sleepTimeout}
	exit 0
}

wait_infinitely() {

  # doing nothing while waiting for processor to kill me
  while true; do
    sleep 0.1
    read
  done < /dev/stdin
}

case $EVENTBODY in
  "sleep"*) sleep_and_exit ;;
  *)        wait_infinitely ;;
esac
