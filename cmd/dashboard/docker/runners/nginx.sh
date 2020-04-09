#!/usr/bin/env sh

echo "Starting nginx"
/usr/sbin/nginx -g "daemon off;" &> /var/log/nginx/logs.log
