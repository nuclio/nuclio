# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
FROM golang:1.11 AS build
LABEL maintainer "golang-dev@googlegroups.com"

ENV CGO_ENABLED=0

# BEGIN deps (run `make update-deps` to update)

# Repo cloud.google.com/go at cc28e31 (2018-11-07)
ENV REV=cc28e317d9ccd190960a456486e63767532be6ba
RUN go get -d cloud.google.com/go/compute/metadata &&\
    (cd /go/src/cloud.google.com/go && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/golang/protobuf at 75dceb1 (2018-11-05)
ENV REV=75dceb112b174be156fa9952e66d3e99945572b4
RUN go get -d github.com/golang/protobuf/proto `#and 5 other pkgs` &&\
    (cd /go/src/github.com/golang/protobuf && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/google/go-github at 01a88ae (2018-10-30)
ENV REV=01a88aee87606a81629924290413387b3ca30b00
RUN go get -d github.com/google/go-github/github &&\
    (cd /go/src/github.com/google/go-github && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/google/go-querystring at 44c6ddd (2018-09-16)
ENV REV=44c6ddd0a2342c386950e880b658017258da92fc
RUN go get -d github.com/google/go-querystring/query &&\
    (cd /go/src/github.com/google/go-querystring && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/gregjones/httpcache at 9cad4c3 (2018-03-06)
ENV REV=9cad4c3443a7200dd6400aef47183728de563a38
RUN go get -d github.com/gregjones/httpcache &&\
    (cd /go/src/github.com/gregjones/httpcache && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo go4.org at 417644f (2018-08-09)
ENV REV=417644f6feb5ed3a356ca5d6d8e3a3fac7dfd33f
RUN go get -d go4.org/strutil `#and 2 other pkgs` &&\
    (cd /go/src/go4.org && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/net at 1c5f79c (2018-11-07)
ENV REV=1c5f79cfb1642860bbe00b6cfce66700c01e04f6
RUN go get -d golang.org/x/net/context `#and 8 other pkgs` &&\
    (cd /go/src/golang.org/x/net && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/oauth2 at f42d051 (2018-11-06)
ENV REV=f42d05182288abf10faef86d16c0d07b8d40ea2d
RUN go get -d golang.org/x/oauth2 `#and 2 other pkgs` &&\
    (cd /go/src/golang.org/x/oauth2 && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/sync at 1d60e46 (2018-03-14)
ENV REV=1d60e4601c6fd243af51cc01ddf169918a5407ca
RUN go get -d golang.org/x/sync/errgroup &&\
    (cd /go/src/golang.org/x/sync && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/text at 6f44c5a (2018-10-30)
ENV REV=6f44c5a2ea40ee3593d98cdcc905cc1fdaa660e2
RUN go get -d golang.org/x/text/secure/bidirule `#and 4 other pkgs` &&\
    (cd /go/src/golang.org/x/text && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/time at fbb02b2 (2018-04-12)
ENV REV=fbb02b2291d28baffd63558aa44b4b56f178d650
RUN go get -d golang.org/x/time/rate &&\
    (cd /go/src/golang.org/x/time && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo grpc.go4.org at 11d0a25 (2017-06-09)
ENV REV=11d0a25b491971beb5a4625ea7856a3c4afaafa5
RUN go get -d grpc.go4.org `#and 11 other pkgs` &&\
    (cd /go/src/grpc.go4.org && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Optimization to speed up iterative development, not necessary for correctness:
RUN go install cloud.google.com/go/compute/metadata \
	github.com/golang/protobuf/proto \
	github.com/golang/protobuf/ptypes \
	github.com/golang/protobuf/ptypes/any \
	github.com/golang/protobuf/ptypes/duration \
	github.com/golang/protobuf/ptypes/timestamp \
	github.com/google/go-github/github \
	github.com/google/go-querystring/query \
	github.com/gregjones/httpcache \
	go4.org/strutil \
	go4.org/types \
	golang.org/x/net/context \
	golang.org/x/net/context/ctxhttp \
	golang.org/x/net/http/httpguts \
	golang.org/x/net/http2 \
	golang.org/x/net/http2/hpack \
	golang.org/x/net/idna \
	golang.org/x/net/internal/timeseries \
	golang.org/x/net/trace \
	golang.org/x/oauth2 \
	golang.org/x/oauth2/internal \
	golang.org/x/sync/errgroup \
	golang.org/x/text/secure/bidirule \
	golang.org/x/text/transform \
	golang.org/x/text/unicode/bidi \
	golang.org/x/text/unicode/norm \
	golang.org/x/time/rate \
	grpc.go4.org \
	grpc.go4.org/codes \
	grpc.go4.org/credentials \
	grpc.go4.org/grpclog \
	grpc.go4.org/internal \
	grpc.go4.org/metadata \
	grpc.go4.org/naming \
	grpc.go4.org/peer \
	grpc.go4.org/stats \
	grpc.go4.org/tap \
	grpc.go4.org/transport
# END deps

COPY . /go/src/golang.org/x/build/

RUN go install golang.org/x/build/cmd/gopherbot


FROM scratch
LABEL maintainer "golang-dev@googlegroups.com"

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/gopherbot /
ENTRYPOINT ["/gopherbot"]
