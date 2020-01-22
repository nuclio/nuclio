# KDC Intergation Test Instance for TEST.GOKRB5

DO NOT USE THIS CONTAINER FOR ANY PRODUCTION USE!!!

To run:
```bash
docker run -v /etc/localtime:/etc/localtime:ro -p 58:88 -p 58:88/udp --rm --name gokrb5-kdc-shorttickets jcmturner/gokrb5:kdc-shorttickets &
```

To build:
```bash
docker build -t jcmturner/gokrb5:kdc-shorttickets --force-rm=true --rm=true .
docker push jcmturner/gokrb5:kdc-shorttickets
```


