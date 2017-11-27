FROM golang:1.9-stretch

RUN apt-get update
RUN apt-get install -y \
    curl \
    g++ \
    make \
    pkg-config \
    python
RUN curl -LO https://nodejs.org/dist/v9.2.0/node-v9.2.0.tar.gz
RUN tar xzf node-v9.2.0.tar.gz
WORKDIR node-v9.2.0
RUN ./configure --prefix=/opt --shared
RUN make -j4 install
RUN ln -s /opt/lib/libnode.so.59 /opt/lib/libnode.so
# Make libnode available
RUN ldconfig /opt/lib
# Add pkg-config support
COPY pkg/processor/build/runtime/nodejs/docker/nodejs.pc /usr/share/pkgconfig

RUN go get github.com/nuclio/nuclio-sdk
WORKDIR /go/src/github.com/nuclio/nuclio

ARG NUCLIO_NODE_VERSION=9.2
ARG NUCLIO_NODE_OS=stretch
# generate a version file
ARG NUCLIO_VERSION_INFO_FILE_CONTENTS

RUN mkdir -p /etc/nuclio 
RUN echo ${NUCLIO_VERSION_INFO_FILE_CONTENTS} > /etc/nuclio/version_info.json

WORKDIR /go/src/github.com/nuclio/nuclio
COPY . .
RUN go build -tags nodejs -a -ldflags="-s -w" -o /processor cmd/processor/main.go
