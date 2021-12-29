# Deploying Functions from a Dockerfile

This tutorial guides you through the process of deploying functions whose build process is solely defined in a user-supplied Dockerfile. The tutorial assumes that you followed the [source-based deployment tutorial](/docs/tasks/deploying-functions.md), which provides an introduction to function signatures, configuration, and more.

#### In this document

- [How is this different from source-based deploys?](#how-is-this-different-from-source-based-deploys)
- [Building a function with Docker](#building-a-function-with-docker)
- [Deploying a function built with Docker](#deploying-a-function-built-with-docker)

## How is this different from source-based deploys?

A Nuclio function is, at the end of the build process, a container image. This image includes all the components required to run the function, except for the function configuration:

- Processor binary
- Per-runtime shim layer (for example, a Python application that communicates with the processor on one side and the user's code on the other)
- User code

To be able to generate such an image using Docker, you must provide a Dockerfile. When you provide Nuclio's build process with source code (be it from a local directory, a local file, or a URL pointing to an archive), Nuclio generates this Dockerfile for you. Prior to version 0.5.0 of Nuclio, this generation process required formatting and templating, and therefore happened in the code. In version 0.5.0, a Dockerfile-based build process was introduced, modifying the previous build recipe into one that can be solely represented in a Dockerfile. This opened up a new build method in which the user either provides a Dockerfile, or even more extremely - builds the function image himself using only `docker build`.

Dockerfile deployment isn't better than source-based deployment; it's just another way for users to create function images. Even prior to version 0.5.0, Nuclio had features that allowed users to inject build-time parameters, like `spec.build.commands` for running `apk`, `apt-get`, `pip`, and other package providers. However, some users may prefer to handle build themselves, using the tools they know and love.

> **Note:** While the process itself is offered as an alternative, many good things came from this feature. Most notably, prior to version 0.5.0, users were limited to using pre-baked "alpine" or "jessie" base images. Now, source-based and Dockerfile-based builds can provide any base image, as long as this base image contains the runtime environment suitable for the runtime; (for example, to run Python functions, the image must contain a Python interpreter).

## Building a function with Docker

Create an empty directory in your preferred location. Then, download a simple Go handler to this directory:

```sh
curl -LO https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go
```

Now, create a Dockerfile by following the guidelines in the [Go reference](/docs/reference/runtimes/golang/golang-reference.md#dockerfile).

> **Note:** Future versions of `nuctl` will automate creating these blueprints through something like `nuctl create blueprint --runtime python:3.6`, which will create a Dockerfile, a **function.yaml** file, and an empty Python handler.

The Dockerfile will look something like this:

```
ARG NUCLIO_LABEL=latest
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

This multi-stage Dockerfile uses three `FROM` directives:

1. `FROM nuclio/uhttpc:0.0.1-amd64` - used for providing an [open-source health-check related binary](https://github.com/nuclio/uhttpc) (basically, a self contained curl). This is used for the "local" platform. You don't need this or the `HEALTHCHECK` platform if you plan on running only on Kubernetes.
2. `FROM alpine:3.15` - the base image on which the final processor image will run.
3. `FROM nuclio/handler-builder-golang-onbuild` - this is where it gets interesting: while every runtime needs the processor binary, each runtime must also provide a unique set of artifacts. Interpreter-based runtimes, like Python and NodeJS, simply need to provide the shim layer and the user's code. However, compiled runtimes (Go, Java, .NET Core) must compile the user's code into a binary. This is done with a set of `ONBUILD` directives in the `onbuild` image. You provide the source, and the base image will do everything that is required to provide you with the artifact at the expected location. In this tutorial, by simply using `FROM nuclio/handler-builder-golang-onbuild` and providing Go source code, you will build a Go plugin that will reside at `/opt/nuclio/handler.so`. All you have to do is copy the plugin to the proper location in you final processor image.

It is up to you to customize this Dockerfile, if you so choose (for example, by adding `RUN` directives that add dependencies), but all provided Dockerfiles are ready to go. Go ahead and build the function; you only need the **Dockerfile** and **helloworld.go**:

```sh
docker build -t helloworld-from-df .
```

> **Note:** Each runtime has a different Dockerfile. Consult the appropriate [runtime reference documents](/docs/reference/runtimes) to understand the specific nuances.

## Deploying a function built with Docker

Now that you have a function image, you can use Nuclio's ability to [deploy pre-built functions](/docs/tasks/deploying-pre-built-functions.md). This is no different than if you had used `nuctl build` to build the function image:

```sh
nuctl deploy helloworld --run-image helloworld-from-df:latest \
    --runtime golang \
    --handler main:Handler \
    --platform local
```

Finally, you can invoke your function:
```sh
nuctl invoke helloworld --platform local
```

