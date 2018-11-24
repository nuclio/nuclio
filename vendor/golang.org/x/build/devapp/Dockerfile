# Copyright 2017 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
FROM golang:1.10 AS builder
LABEL maintainer "golang-dev@googlegroups.com"

ENV CGO_ENABLED=0

# BEGIN deps (run `make update-deps` to update)

# Repo cloud.google.com/go at c41d1f0 (2018-04-11)
ENV REV=c41d1f0620c95e5a241910263809a53a5425af0d
RUN go get -d cloud.google.com/go/compute/metadata `#and 7 other pkgs` &&\
    (cd /go/src/cloud.google.com/go && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/golang/protobuf at e09c5db (2018-03-28)
ENV REV=e09c5db296004fbe3f74490e84dcd62c3c5ddb1b
RUN go get -d github.com/golang/protobuf/proto `#and 6 other pkgs` &&\
    (cd /go/src/github.com/golang/protobuf && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/google/go-github at 60131d1 (2018-03-29)
ENV REV=60131d157172230b327fee58845dfcae3dee57b4
RUN go get -d github.com/google/go-github/github &&\
    (cd /go/src/github.com/google/go-github && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/google/go-querystring at 53e6ce1 (2017-01-11)
ENV REV=53e6ce116135b80d037921a7fdd5138cf32d7a8a
RUN go get -d github.com/google/go-querystring/query &&\
    (cd /go/src/github.com/google/go-querystring && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/googleapis/gax-go at de2cc08 (2018-03-29)
ENV REV=de2cc08e690b99dd3f7d19937d80d3d54e04682f
RUN go get -d github.com/googleapis/gax-go &&\
    (cd /go/src/github.com/googleapis/gax-go && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo github.com/gregjones/httpcache at 2bcd89a (2017-11-19)
ENV REV=2bcd89a1743fd4b373f7370ce8ddc14dfbd18229
RUN go get -d github.com/gregjones/httpcache &&\
    (cd /go/src/github.com/gregjones/httpcache && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo go.opencensus.io at 5710257 (2018-04-10)
ENV REV=5710257e78ab540a7c6f0e203009abea60eb97b7
RUN go get -d go.opencensus.io/exporter/stackdriver/propagation `#and 12 other pkgs` &&\
    (cd /go/src/go.opencensus.io && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo go4.org at fba789b (2018-01-03)
ENV REV=fba789b7e39ba524b9e60c45c37a50fae63a2a09
RUN go get -d go4.org/types &&\
    (cd /go/src/go4.org && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/crypto at d644981 (2018-04-11)
ENV REV=d6449816ce06963d9d136eee5a56fca5b0616e7e
RUN go get -d golang.org/x/crypto/acme `#and 2 other pkgs` &&\
    (cd /go/src/golang.org/x/crypto && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/net at 61147c4 (2018-04-06)
ENV REV=61147c48b25b599e5b561d2e9c4f3e1ef489ca41
RUN go get -d golang.org/x/net/context `#and 8 other pkgs` &&\
    (cd /go/src/golang.org/x/net && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/oauth2 at 921ae39 (2018-04-02)
ENV REV=921ae394b9430ed4fb549668d7b087601bd60a81
RUN go get -d golang.org/x/oauth2 `#and 5 other pkgs` &&\
    (cd /go/src/golang.org/x/oauth2 && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/sync at 1d60e46 (2018-03-14)
ENV REV=1d60e4601c6fd243af51cc01ddf169918a5407ca
RUN go get -d golang.org/x/sync/errgroup &&\
    (cd /go/src/golang.org/x/sync && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/text at 7922cc4 (2018-04-10)
ENV REV=7922cc490dd5a7dbaa7fd5d6196b49db59ac042f
RUN go get -d golang.org/x/text/secure/bidirule `#and 4 other pkgs` &&\
    (cd /go/src/golang.org/x/text && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo golang.org/x/time at 6dc1736 (2017-09-27)
ENV REV=6dc17368e09b0e8634d71cac8168d853e869a0c7
RUN go get -d golang.org/x/time/rate &&\
    (cd /go/src/golang.org/x/time && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo google.golang.org/api at 9f7560f (2018-04-10)
ENV REV=9f7560f3b05bd90f33f9d56a449e5afd4dab15b3
RUN go get -d google.golang.org/api/gensupport `#and 9 other pkgs` &&\
    (cd /go/src/google.golang.org/api && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo google.golang.org/genproto at 51d0944 (2018-04-09)
ENV REV=51d0944304c3cbce4afe9e5247e21100037bff78
RUN go get -d google.golang.org/genproto/googleapis/api/annotations `#and 4 other pkgs` &&\
    (cd /go/src/google.golang.org/genproto && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Repo google.golang.org/grpc at 858463a (2018-04-10)
ENV REV=858463ac273fd87e6b2343eda6e96ed3a91e3b20
RUN go get -d google.golang.org/grpc `#and 24 other pkgs` &&\
    (cd /go/src/google.golang.org/grpc && (git cat-file -t $REV 2>/dev/null || git fetch -q origin $REV) && git reset --hard $REV)

# Optimization to speed up iterative development, not necessary for correctness:
RUN go install cloud.google.com/go/compute/metadata \
	cloud.google.com/go/iam \
	cloud.google.com/go/internal \
	cloud.google.com/go/internal/optional \
	cloud.google.com/go/internal/trace \
	cloud.google.com/go/internal/version \
	cloud.google.com/go/storage \
	github.com/golang/protobuf/proto \
	github.com/golang/protobuf/protoc-gen-go/descriptor \
	github.com/golang/protobuf/ptypes \
	github.com/golang/protobuf/ptypes/any \
	github.com/golang/protobuf/ptypes/duration \
	github.com/golang/protobuf/ptypes/timestamp \
	github.com/google/go-github/github \
	github.com/google/go-querystring/query \
	github.com/googleapis/gax-go \
	github.com/gregjones/httpcache \
	go.opencensus.io/exporter/stackdriver/propagation \
	go.opencensus.io/internal \
	go.opencensus.io/internal/tagencoding \
	go.opencensus.io/plugin/ochttp \
	go.opencensus.io/plugin/ochttp/propagation/b3 \
	go.opencensus.io/stats \
	go.opencensus.io/stats/internal \
	go.opencensus.io/stats/view \
	go.opencensus.io/tag \
	go.opencensus.io/trace \
	go.opencensus.io/trace/internal \
	go.opencensus.io/trace/propagation \
	go4.org/types \
	golang.org/x/crypto/acme \
	golang.org/x/crypto/acme/autocert \
	golang.org/x/net/context \
	golang.org/x/net/context/ctxhttp \
	golang.org/x/net/http2 \
	golang.org/x/net/http2/hpack \
	golang.org/x/net/idna \
	golang.org/x/net/internal/timeseries \
	golang.org/x/net/lex/httplex \
	golang.org/x/net/trace \
	golang.org/x/oauth2 \
	golang.org/x/oauth2/google \
	golang.org/x/oauth2/internal \
	golang.org/x/oauth2/jws \
	golang.org/x/oauth2/jwt \
	golang.org/x/sync/errgroup \
	golang.org/x/text/secure/bidirule \
	golang.org/x/text/transform \
	golang.org/x/text/unicode/bidi \
	golang.org/x/text/unicode/norm \
	golang.org/x/time/rate \
	google.golang.org/api/gensupport \
	google.golang.org/api/googleapi \
	google.golang.org/api/googleapi/internal/uritemplates \
	google.golang.org/api/googleapi/transport \
	google.golang.org/api/internal \
	google.golang.org/api/iterator \
	google.golang.org/api/option \
	google.golang.org/api/storage/v1 \
	google.golang.org/api/transport/http \
	google.golang.org/genproto/googleapis/api/annotations \
	google.golang.org/genproto/googleapis/iam/v1 \
	google.golang.org/genproto/googleapis/rpc/code \
	google.golang.org/genproto/googleapis/rpc/status \
	google.golang.org/grpc \
	google.golang.org/grpc/balancer \
	google.golang.org/grpc/balancer/base \
	google.golang.org/grpc/balancer/roundrobin \
	google.golang.org/grpc/channelz \
	google.golang.org/grpc/codes \
	google.golang.org/grpc/connectivity \
	google.golang.org/grpc/credentials \
	google.golang.org/grpc/encoding \
	google.golang.org/grpc/encoding/proto \
	google.golang.org/grpc/grpclb/grpc_lb_v1/messages \
	google.golang.org/grpc/grpclog \
	google.golang.org/grpc/internal \
	google.golang.org/grpc/keepalive \
	google.golang.org/grpc/metadata \
	google.golang.org/grpc/naming \
	google.golang.org/grpc/peer \
	google.golang.org/grpc/resolver \
	google.golang.org/grpc/resolver/dns \
	google.golang.org/grpc/resolver/passthrough \
	google.golang.org/grpc/stats \
	google.golang.org/grpc/status \
	google.golang.org/grpc/tap \
	google.golang.org/grpc/transport
# END deps

COPY . /go/src/golang.org/x/build/

RUN go install golang.org/x/build/devapp

FROM scratch
LABEL maintainer "golang-dev@googlegroups.com"
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/bin/devapp /
COPY devapp/static /static
COPY devapp/templates /templates
ENTRYPOINT ["/devapp"]
