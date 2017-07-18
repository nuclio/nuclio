FROM alpine:3.6

RUN apk add --no-cache ca-certificates

COPY _output/controller /usr/local/bin

CMD [ "controller" ]
