# Python Reference

This document describes the specific Python build and deploy configurations.

#### In this document

- [Function and handler](#function-and-handler)
- [Dockerfile](#dockerfile)
- [Python runtime 2.7 EOL](#python-runtime-27-eol)
- [Introducing Python runtimes 3.7, 3.8 and 3.9](#introducing-python-runtimes-37-38-and-39)
- [Function configuration](#function-configuration)
- [Build and execution](#build-and-execution)

## Function and handler

```python
import nuclio_sdk

def handler(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    context.logger.info_with('Invoked', method=event.method)
    return "Hello, from Nuclio :]"
```

The `handler` field is of the form `<package>:<entrypoint>`, where `<package>` is a dot (`.`) separated path (for example, `foo.bar` equates to `foo/bar.py`) and `<entrypoint>` is the function name. In the example above, the handler is `main:handler`, assuming the file is named `main.py`.

For asynchronous support (e.g.: `asyncio`), you may want to decorate your function handle with `async`

Important to note:
  - Nuclio, at the moment, does not support concurrent requests handling for a single working. Each working may handle
    one request at a time, for more information see [here](/docs/concepts/architecture.md#runtime-engine).
  - However, using an async handler can still be beneficial in some scenarios; Since the event loop would keep running while listening on more incoming requests, it allows functions to asynchronously perform
    I/O bound background tasks.

```python
import asyncio

import nuclio_sdk

async def handler(context: nuclio_sdk.Context, event: nuclio_sdk.Event):
    context.logger.info_with('Updating db in background', event_body=event.body.decode())
    asyncio.create_task(update_db(context, event))    
    return "Hello, from Nuclio :]"

async def update_db(context, event):
    context.db.update_record(event.body)
```

## Dockerfile

Following is sample Dockerfile code for deploying a Python function. For more information, see [Deploying Functions from a Dockerfile](/docs/tasks/deploy-functions-from-dockerfile.md).

> **Note:** Make sure to replace `my-function-code` and `my-function.yaml` in the following example with the names of your function and function-configuration file.

```
ARG NUCLIO_LABEL=1.6.0
ARG NUCLIO_ARCH=amd64
ARG NUCLIO_BASE_IMAGE=python:3.8
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
COPY --from=processor /home/nuclio/bin/py*-whl/* /opt/nuclio/whl/
COPY --from=uhttpc /home/nuclio/bin/uhttpc /usr/local/bin/uhttpc

# Install pip (if missing) + nuclio python sdk and its dependencies
# Using "python -m" to ensure the given interpreter has all dependencies installed in cases
# .. where there is more than one python interpreter and global pip is attached to the other interpreter
RUN python /opt/nuclio/whl/$(basename /opt/nuclio/whl/pip-*.whl)/pip install pip --no-index --find-links /opt/nuclio/whl \
 && python -m pip install nuclio-sdk msgpack --no-index --find-links /opt/nuclio/whl

# Readiness probe
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://127.0.0.1:8082/ready || exit 1

# Copy the function code, including the handler directory to /opt/nuclio
COPY . /opt/nuclio

# Run processor with configuration and platform configuration
CMD [ "processor" ]
```

<a id="python-runtime-27-eol"></a>
## Python runtime 2.7 EOL

As of Jan 2020, [Python 2.7 is no longer being maintained](https://www.python.org/doc/sunset-python-2/), and it has now
also reached its End Of Life in Nuclio as well, and thus removed as a supported runtime from the mainline Nuclio
releases. Starting from [Nuclio 1.6.0](https://github.com/nuclio/nuclio/releases/tag/1.6.0) you would not be able to
deploy any Nuclio function using Python 2.7 runtime.

To keep using latest Nuclio, and reach better performance and message throughput, we strongly suggest migrating your
code to the newer [Python 3.7, 3.8 and 3.9 runtimes](#introducing-python-runtimes-37-38-and-39), if you haven't already.

<a id="introducing-python-runtimes-37-38-and-39"></a>
## Introducing Python runtimes 3.7, 3.8 and 3.9

Nuclio officially supports python 3.7, 3.8 and python 3.9 (along with good-old python 3.6) as stand-alone runtimes. Along
with simply bumping the python versions, some changes to the internal function processor were made, to take advantage of
new language features and newer packages.

Key differences and changes:

- Python 3.8+ is 5%-8% faster than Python 3.6 for small sized event messages.
- Python 3.7, 3.8 and 3.9 base images are `python:3.7`, `python:3.8` and `python:3.9`, respectively.
- Events metadata, such as headers, path, method, etc are now byte-strings. This may incur changes in your code to refer
  to the various (now) byte-string event properties correctly in the new runtimes. e.g.: Simple code snipped which
  worked on python 2.7 and 3.6, using some event metadata, such as `event.path` -

  ```python
  def handler(context, event):
    if event.path == "/do_something":
      return "I'm doing something..."
  ```

  In Python 3.7+ runtimes, the matching `event.path` property is now a `byte-string` instead of an old `string`,
  The new snippet will look like this:

  ```python
  def handler(context, event):
    if event.path == b"/do_something":
      return "I'm doing something..."
  ```

  > Note: To decode all incoming event byte-strings automatically by the nuclio python wrapper, set the function
  > environment variable: `NUCLIO_PYTHON_DECODE_EVENT_STRINGS=true`. Enabling event strings decoding the Nuclio python
  > wrapper might fail to handle events with non-utf8 metadata contents. Part of the motivation for the change was
  > to gain robustness against such cases and transfer the encoding task to user-code.

> Note: Python 3.6 runtimes is left unchanged.

<a id="function-configuration"></a>
## Function configuration

Your function-configuration file (for example, **my-function.yaml** for the [example Dockerfile](#dockerfile)) must include the name of your handler function and Python runtime. For more information, see the [function-configuration reference](/docs/reference/function-configuration/function-configuration-reference.md). For example:

```yaml
meta:
  name: "my-function"
spec:
  handler: main:handler
  runtime: python:3.8
  triggers:
    myHttpTrigger:
      maxWorkers: 1
      kind: "http"

```

<a id="build-and-execution"></a>
## Build and execution

Following are example commands for building and running the latest version of a `my-function` function that's listening on port 8090.

You may replace the function name, and the published port number, as needed:

```sh
docker build --tag my-function:latest .

docker run \
  --rm \
  --detach \
  --volume /path/to/function.yaml:/etc/nuclio/config/processor/processor.yaml \
  --name my-function \
  --publish 8090:8080 \
  my-function:latest
```

## Portable execution

If you have baked in your function configuration (aka `function.yaml`) onto the function container image, 
you *do not* have to volumize it during execution, but rather explicitly overriding the function configuration path; e.g.:

```sh
docker run \
  --rm \
  --detach \
  --name my-function \
  --publish 8090:8080 \
  my-function:latest \
  processor --config /opt/nuclio/function.yaml
```

That way, you can build your function once, deploy it as much as desired, 
without being needed to volumize the function configuration upon each deployment.
