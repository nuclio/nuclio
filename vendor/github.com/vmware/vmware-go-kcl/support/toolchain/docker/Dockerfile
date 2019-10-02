FROM golang:1.12
ENV PATH /go/bin:/src/bin:/root/go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go:/src
RUN go get -v github.com/alecthomas/gometalinter && \
    go get -v golang.org/x/tools/cmd/... && \
    go get -v github.com/FiloSottile/gvt && \
    go get github.com/securego/gosec/cmd/gosec/... && \
    go get github.com/derekparker/delve/cmd/dlv && \
    gometalinter --install && \
    chmod -R a+rw /go