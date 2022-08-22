# Copyright 2017 The Nuclio Authors.
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
#!/usr/bin/env sh

_term() {
  echo "Signal caught ... cleaning up"
  kill -TERM "$child" 2>/dev/null
}

trap _term TERM
trap _term INT

echo "Running in parallel"

# citation
# ensure each runner gets a job slot
# buffer output on line basis
# exit when the first job fails, kill all running jobs.
# upon unexpected termination, signal jobs before killing (signal, timeout)
# execute all *.sh files in parallel
parallel \
        --will-cite \
        --jobs 0 \
        --line-buffer \
        --halt now,fail=1 \
        --termseq INT,200,TERM,100,KILL,25 \
        '{}' ::: /runners/*.sh &

child=$!
wait "$child"
echo "Exiting"
