FROM golang:1.8

COPY nuclio-builder.sh /usr/local/bin/nuclio-builder

RUN mkdir -p /go/src/github.com/nuclio/nuclio
WORKDIR /go/src/github.com/nuclio/nuclio

ONBUILD COPY . /go/src/github.com/nuclio/nuclio
ONBUILD RUN nuclio-builder
