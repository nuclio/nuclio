# NodeJS Reference

This document describes the specific NodeJS build and deploy configurations.

#### In this document

- [Function and handler](#function-and-handler)
- [Dockerfile](#dockerfile)

## Function and handler

```js
exports.handler = function(context, event) {
    context.callback('');
};
```

The `handler` field is of the form `<package>:<entrypoint>`, where `<package>` is a dot (`.`) separated path (for example, `foo.bar` equates to `foo/bar.js`) and `<entrypoint>` is the function name. In the example above, the handler is `handler:handler`, assuming the file is named `handler.js`.
> Note: A temporary limitation mandates that the file be named `handler.js`.

## Dockerfile

See [Deploying Functions from a Dockerfile](/docs/tasks/deploy-functions-from-dockerfile.md).

```
ARG NUCLIO_LABEL=0.5.6
ARG NUCLIO_ARCH=amd64
ARG NUCLIO_BASE_IMAGE=node:10.3-alpine
ARG NUCLIO_ONBUILD_IMAGE=nuclio/handler-builder-nodejs-onbuild:${NUCLIO_LABEL}-${NUCLIO_ARCH}

# Supplies processor uhttpc, used for healthcheck
FROM nuclio/uhttpc:0.0.1-amd64 as uhttpc

# Supplies processor binary, wrapper
FROM ${NUCLIO_ONBUILD_IMAGE} as processor

# From the base image
FROM ${NUCLIO_BASE_IMAGE}

# Copy required objects from the suppliers
COPY --from=processor /home/nuclio/bin/processor /usr/local/bin/processor
COPY --from=processor /home/nuclio/bin/wrapper.js /opt/nuclio/wrapper.js
COPY --from=uhttpc /home/nuclio/bin/uhttpc /usr/local/bin/uhttpc

# Copy the handler directory to /opt/nuclio
COPY . /opt/nuclio

# Readiness probe
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://127.0.0.1:8082/ready || exit 1

# Set node modules path
ENV NODE_PATH=/usr/local/lib/node_modules

# Run processor with configuration and platform configuration
CMD [ "processor", "--config", "/etc/nuclio/config/processor/processor.yaml", "--platform-config", "/etc/nuclio/config/platform/platform.yaml" ]
```

