#!/bin/bash

rm /etc/localtime
ln -s /usr/share/zoneinfo/Europe/London /etc/localtime

mkdir -p /var/log/kerberos
cp /vagrant/krb5.conf /etc/krb5.conf
echo "10.80.88.88 kdc.test.gokrb5" >> /etc/hosts
echo "10.80.88.90 host.test.gokrb5" >> /etc/hosts

sudo apt-get update && sudo apt-get install -y krb5-user ntp && apt-get upgrade -y

reboot
