#!/usr/bin/env sh

echo "Starting nginx"
/usr/sbin/nginx -g "daemon off;" 2>&1 /tmp/nginx-logs.log
