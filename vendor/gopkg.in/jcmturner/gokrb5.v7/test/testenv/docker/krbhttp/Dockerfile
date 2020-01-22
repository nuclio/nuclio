FROM centos:latest
MAINTAINER Jonathan Turner <jt@jtnet.co.uk>

EXPOSE 80 443
ENV LANG C
ENV KRB5RCACHEDIR=/var/tmp
ENV KRB5RCACHETYPE=dfl
ENTRYPOINT ["/usr/sbin/httpd", "-DFOREGROUND"]

RUN yum install -y \
  httpd \
  mod_auth_kerb \
  mod_auth_gssapi \
  mod_session \
  mod_ssl \
  tcpdump krb5-workstation vim \
  && yum update -y && yum clean all

RUN mkdir /var/www/html/modkerb && mkdir /var/www/html/modgssapi
ADD httpd-krb5.conf /etc/httpd/conf.d/
ADD index.html /var/www/html/modkerb/index.html
ADD index.html /var/www/html/modgssapi/index.html
ADD krb5.conf /etc/krb5.conf
ADD http.testtab /etc/httpd/
ADD host.testtab /etc/krb5.keytab
#RUN ln -sf /dev/stdout /var/log/httpd/access_log && \
# ln -sf /dev/stdout /var/log/httpd/ssl_access_log && \
# ln -sf /dev/stdout /var/log/httpd/ssl_request_log && \
# ln -sf /dev/stderr /var/log/httpd/error_log && \
# ln -sf /dev/stderr /var/log/httpd/ssl_error_log


