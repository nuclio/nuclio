#!/bin/bash

set -e

KEY=$1
if [ "$KEY" = "" ]; then
        echo "usage: ./setup.bash <BUILDKEY>" >&2
        exit 2
fi

echo $KEY > /root/.gobuildkey

apt-get update
apt-get upgrade
apt-get install strace libc6-dev gcc

cd /etc/systemd/system
cat >buildlet.service <<EOF
[Unit]
Description=Go builder buildlet
After=network.target

[Install]
WantedBy=network-online.target

[Service]
Type=simple
ExecStartPre=/bin/sh -c 'cd /usr/local/bin; /usr/bin/curl -R -f -z buildlet-stage0 -o buildlet-stage0 https://storage.googleapis.com/go-builder-data/buildlet-stage0.linux-ppc64 && chmod +x buildlet-stage0'
ExecStart=/usr/local/bin/buildlet-stage0
Restart=always
RestartSec=2
StartLimitInterval=0
EOF

systemctl enable buildlet.service
systemctl start buildlet.service
