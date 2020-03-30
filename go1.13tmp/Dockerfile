################
#   PROCESSOR  #
################
FROM golang:1.14-alpine3.11 as processor

RUN apk --update --no-cache add git gcc musl-dev file

WORKDIR /nuclio

ADD go.mod go.sum ./

RUN go mod download

# copy source tree
ADD pkg ./pkg
ADD cmd ./cmd
ADD test ./test

# compile processor
RUN GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-s -w" -o /home/nuclio/bin/processor cmd/processor/main.go

################
#    PLUGIN    #
################

FROM golang:1.14-alpine3.11

RUN apk --update --no-cache add git gcc musl-dev file

# Store processor binary
COPY --from=processor /home/nuclio/bin/processor /home/nuclio/bin/processor

# Store processor go build artifacts
COPY --from=processor /nuclio/go.mod /processor_go.mod
COPY --from=processor /nuclio/go.sum /processor_go.sum

# Build plugin
WORKDIR /handler

# copy handler
ARG HANDLER
ARG NUCLIO_BUILD_OFFLINE

RUN mkdir -p /etc/nuclio/config/processor

# copying processor config
ADD ./go1.13tmp/processor.yaml /etc/nuclio/config/processor/

ADD ./go1.13tmp/handler/${HANDLER} .
ADD ./go1.13tmp/handler/moduler.sh .

RUN chmod +x moduler.sh && ./moduler.sh

# creating plugin
RUN GOOS=linux GOARCH=amd64 go build -mod=mod -buildmode=plugin -o /opt/nuclio/handler.so .

# DOD
CMD ["/home/nuclio/bin/processor"]
