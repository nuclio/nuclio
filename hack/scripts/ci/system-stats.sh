#!/usr/bin/env sh
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
#

echo "Printing System Stats"
echo "-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-"
echo "## Memory ##"
echo "$ free -h"
free -h
echo "--------------------------"
echo "$ ps aux --sort=-%mem | head -n 20"
ps aux --sort=-%mem | head -n 20
echo "--------------------------"
echo "$ top -b -n 1 | head -n 20"
top -b -n 1 | head -n 20
echo "-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-"
echo "## CPU ##"
echo "$ lscpu"
lscpu
echo "--------------------------"
echo "$ ps aux | sort -k 3 -r | head -n 20"
ps aux | sort -k 3 -r | head -n 20
echo "--------------------------"
echo "$ mpstat -P ALL"
mpstat -P ALL
echo "-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-"
echo "## Disk ##"
echo "$ df --human-readable"
df --human-readable
echo "-----------"
