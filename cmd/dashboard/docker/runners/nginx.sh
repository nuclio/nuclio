#!/usr/bin/env sh

echo "Starting nginx"
/usr/sbin/nginx -g "daemon off;" &> /tmp/nginx-logs.log
