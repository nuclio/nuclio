#!/usr/bin/env sh

_term() {
  echo "Signal caught ... cleaning up"
  kill -TERM "$child" 2>/dev/null
}

trap _term SIGTERM
trap _term SIGINT

echo "Running in parallel"

parallel --citation

# ensure each runner gets a jobslot
# buffer output on line basis
# exit when the first job fails, kill all running jobs.
# upon unexpected termination, signal jobs before killing (signal, timeout)
# execute all *.sh files in parallel
parallel \
        --jobs 0 \
        --line-buffer \
        --halt now,fail=1 \
        --termseq INT,200,TERM,100,KILL,25 \
        '{}' ::: /runners/*.sh &

child=$!
wait "$child"
echo "Exiting"
