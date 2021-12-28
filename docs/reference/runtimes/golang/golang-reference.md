# Golang (Go) Reference

This document describes specific Golang (Go) build and deploy configurations.

#### In this document

- [Function and handler](#function-and-handler)
- [Dockerfile](#dockerfile)

## Function and handler

```go
package main

import (
    "github.com/nuclio/nuclio-sdk-go"
)

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
    return nil, nil
}
```

The function package must be `main`, because the code compiles into a Go plugin. The `handler` field can be empty, as the Go runtime supports auto-handler detection by parsing the AST and looking for an exported function with the expected signature. Should you want to provide a handler for consistency, it should be of the form `<package>:<entrypoint>`. In the example above, the handler is `main:Handler`.

## Dockerfile

See [Deploying Functions from a Dockerfile](/docs/tasks/deploy-functions-from-dockerfile.md).

```
ARG NUCLIO_LABEL=0.5.6
ARG NUCLIO_ARCH=amd64
ARG NUCLIO_BASE_IMAGE=alpine:3.15
ARG NUCLIO_ONBUILD_IMAGE=nuclio/handler-builder-golang-onbuild:${NUCLIO_LABEL}-${NUCLIO_ARCH}-alpine

# Supplies processor uhttpc, used for healthcheck
FROM nuclio/uhttpc:0.0.1-amd64 as uhttpc

# Builds source, supplies processor binary and handler plugin
FROM ${NUCLIO_ONBUILD_IMAGE} as builder

# From the base image
FROM ${NUCLIO_BASE_IMAGE}

# Copy required objects from the suppliers
COPY --from=builder /home/nuclio/bin/processor /usr/local/bin/processor
COPY --from=builder /home/nuclio/bin/handler.so /opt/nuclio/handler.so
COPY --from=uhttpc /home/nuclio/bin/uhttpc /usr/local/bin/uhttpc

# Readiness probe
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://127.0.0.1:8082/ready || exit 1

# Run processor with configuration and platform configuration
CMD [ "processor" ]
```

