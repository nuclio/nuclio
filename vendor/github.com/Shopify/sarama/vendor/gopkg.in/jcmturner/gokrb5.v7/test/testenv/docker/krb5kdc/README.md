# KDC Intergation Test Instance for TEST.GOKRB5

DO NOT USE THIS CONTAINER FOR ANY PRODUCTION USE!!!

To run:
```bash
docker run -v /etc/localtime:/etc/localtime:ro -p 88:88 -p 88:88/udp -p 464:464 -p 464:464/udp --rm --name gokrb5-kdc-centos-default jcmturner/gokrb5:kdc-centos-default &
```

To build:
```bash
docker build -t jcmturner/gokrb5:kdc-centos-default --force-rm=true --rm=true .
docker push jcmturner/gokrb5:kdc-centos-default
```


