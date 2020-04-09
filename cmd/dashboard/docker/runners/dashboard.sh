#!/usr/bin/env sh

echo "Starting dashboard"
/usr/local/bin/dashboard --docker-key-dir /etc/nuclio/dashboard/registry-credentials --listen-addr :18070 2>&1
