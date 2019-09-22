# Python Reference

This document describes the specific Python build and deploy configurations.

#### In this document

- [Function and handler](#function-and-handler)
- [Dockerfile](#dockerfile)
- [Function configuration](#function-configuration)
- [Build and execution](#build-and-execution)

## Function and handler

```python
def handler(context, event):
    return ""
```

The `handler` field is of the form `<package>:<entrypoint>`, where `<package>` is a dot (`.`) separated path (for example, `foo.bar` equates to `foo/bar.py`) and `<entrypoint>` is the function name. In the example above, the handler is `main:handler`, assuming the file is named `main.py`.

## Dockerfile

Following is sample Dockerfile code for deploying a Python function. For more information, see [Deploying Functions from a Dockerfile](/docs/tasks/deploy-functions-from-dockerfile.md).

> **Note:** Make sure to replace `my-function-code` and `my-function.yaml` in the following example with the names of your function and function-configuration file.

```
ARG NUCLIO_LABEL=1.1.18
ARG NUCLIO_ARCH=amd64
ARG NUCLIO_BASE_IMAGE=python:3.6-jessie
ARG NUCLIO_ONBUILD_IMAGE=quay.io/nuclio/handler-builder-python-onbuild:${NUCLIO_LABEL}-${NUCLIO_ARCH}

# Supplies processor uhttpc, used for healthcheck
FROM nuclio/uhttpc:0.0.1-amd64 as uhttpc

# Supplies processor binary, wrapper
FROM ${NUCLIO_ONBUILD_IMAGE} as processor

# From the base image
FROM ${NUCLIO_BASE_IMAGE}

# Copy required objects from the suppliers
COPY --from=processor /home/nuclio/bin/processor /usr/local/bin/processor
COPY --from=processor /home/nuclio/bin/py /opt/nuclio/
COPY --from=uhttpc /home/nuclio/bin/uhttpc /usr/local/bin/uhttpc

RUN pip install nuclio-sdk msgpack --no-index --find-links /opt/nuclio/whl

# Readiness probe
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://127.0.0.1:8082/ready || exit 1

# USER CONTENT
ADD ./my-function-code /opt/nuclio
ADD ./my-function.yaml /etc/nuclio/config/processor/processor.yaml
# END OF USER CONTENT

# Run processor with configuration and platform configuration
CMD [ "processor" ]
```

<a id="function-configuration"></a>
## Function configuration

Your function-configuration file (for example, **my-function.yaml** for the [example Dockerfile](#dockerfile)) must include the name of your handler function and Python runtime. For more information, see the [function-configuration reference](/docs/reference/function-configuration/function-configuration-reference.md). For example:

```yaml
spec:
  handler: main:handler
  runtime: python:3.6
```

<a id="build-and-execution"></a>
## Build and execution

Following are example commands for building and running the latest version of a `my-function` function that's listening on port 8090; replace the function name and version and the port number, as needed:

```sh
docker build -t my-function:latest .
docker run --name my-function --rm -d -p 8090:8080 my-function:latest
```
