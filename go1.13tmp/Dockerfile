################
#   PROCESSOR  #
################
FROM golang:1.13-alpine3.10 as processor

RUN apk --update --no-cache add git gcc musl-dev file

WORKDIR /go/src/github.com/nuclio/nuclio

ADD go.mod go.sum ./

# build the processor
RUN go mod download

# copy source tree
ADD pkg ./pkg
ADD cmd ./cmd
ADD test ./test

# compile processor
RUN GOOS=linux GOARCH=amd64 go build -mod=vendor -a -installsuffix cgo -ldflags="-s -w" -o /home/nuclio/bin/processor cmd/processor/main.go

################
#    PLUGIN    #
################

FROM golang:1.13-alpine3.10

RUN apk --update --no-cache add git gcc musl-dev file

# Store processor binary
COPY --from=processor /home/nuclio/bin/processor /home/nuclio/bin/processor

# Store processor go build artifacts
COPY --from=processor /go/src/github.com/nuclio/nuclio/vendor /processor_vendor

# Add go.mod and sum if needed (processor's)
ADD go.mod /processor_go.mod
ADD go.sum /processor_go.sum

# Build plugin
WORKDIR /go/src/github.com/nuclio/handler

# copy handler
ARG HANDLER="without_own_go_modules"
ARG NUCLIO_BUILD_OFFLINE="true"

ADD ./go1.13tmp/handler/${HANDLER} ./
ADD ./go1.13tmp/handler/moduler.sh ./

RUN chmod +x moduler.sh && ./moduler.sh

# creating plugin
RUN GOOS=linux GOARCH=amd64 go build -mod=vendor -buildmode=plugin -o /opt/nuclio/handler.so

# creating processor config dir
RUN mkdir -p /etc/nuclio/config/processor

# copying processor config
ADD ./go1.13tmp/processor.yaml /etc/nuclio/config/processor/

# DOD
CMD ["/home/nuclio/bin/processor"]
