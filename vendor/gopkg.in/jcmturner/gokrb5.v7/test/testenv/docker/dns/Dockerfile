FROM debian:latest
MAINTAINER Jonathan Turner <jt@jtnet.co.uk>

EXPOSE 53

ENTRYPOINT ["/var/named/named.sh"]

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get install -y bind9 && \
  mkdir -p /var/named/data && \
  mkdir -p /var/named/dynamic && \
  chown -R bind /var/named && \
  mkdir -p /etc/named && \
  mkdir -p /var/run/named && chown bind /var/run/named

ENV KRB5_KTNAME /etc/named.keytab
ADD files/krb5.conf /etc/krb5.conf
ADD files/krb5.testtab /var/named/data/named.keytab
RUN chown bind:bind /var/named/data/named.keytab && chmod 644 /var/named/data/named.keytab

ADD files/named.sh /var/named/named.sh
RUN chmod 744 /var/named/named.sh

ADD files/etc-named.conf /etc/named.conf
ADD files/gokrb5.conf /etc/named/gokrb5.conf
ADD files/zone-files/db.10 /var/named/data/
ADD files/zone-files/db.test.gokrb5 /var/named/data/