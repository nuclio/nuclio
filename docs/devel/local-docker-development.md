# Development setup for running Nuclio services in docker

This guide will guide you through running the Nuclio services locally using docker, so they will contain your changes when developing in Nuclio.
This is helpful for testing out code changes in a docker environment before committing them.

If you want to be able to debug the actual components locally, see [local-development.md](local-development.md).

## Prerequisites

Ensure that your setup includes the following prerequisite components:

- Linux or OSX
- Docker version 19.03+
- make


## Building

Run `make build` to build all necessary docker images. 
If you change parts of the application and need to rebuild only specific components, you don't need to re-run `make build`: you can also re-build the components that you changed, e.g. with `make dashboard` / `make controller`.

## Running

To execute the dashboard, execute the following script:

```sh
if [[ $(uname -m) -eq "arm64" ]]; then
  ARCH="arm64"
else
  ARCH="amd64"
fi

COMMAND="docker run \
    --rm -p 8070:8070 \
    -v /var/run/docker.sock:/var/run/docker.sock \
    --name nuclio-dashboard \
    -e NUCLIO_DASHBOARD_NO_PULL_BASE_IMAGES='true' \
    quay.io/nuclio/dashboard:latest-$ARCH"

eval "$COMMAND"
```

You can now access the dashboard by opening `localhost:8000` in a browser.

Note: if you want to use `nuctl`, you need to add `--platform local` to every command (or `export NUCTL_PLATFORM="local"`).
