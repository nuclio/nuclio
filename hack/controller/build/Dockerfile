FROM alpine:3.6

RUN apk add --no-cache ca-certificates

ADD _output/bin/controller /usr/local/bin

CMD [ "controller" ]
