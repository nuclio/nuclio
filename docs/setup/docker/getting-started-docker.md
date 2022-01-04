# Getting Started with Nuclio on Docker

Follow this step-by-step guide to set up a Nuclio development environment that uses Docker.

#### In this document

- [Prerequisites](#prerequisites)
- [Run Nuclio](#run-nuclio)
- [What's next](#whats-next)

## Prerequisites

Before starting the set-up procedure, ensure that the following prerequisites are met:

- Your environment has a running [Docker](https://docker.com) daemon.
    To ensure that your Docker daemon is running properly, run the following command with the same user that will be used to execute Nuclio CLI commands:
    ```sh
    docker version
    ```

- The [Nuclio CLI](/docs/reference/nuctl/nuctl.md) (`nuctl`) is installed &mdash; if you wish to [use the CLI to deploy Nuclio functions](#deploy-a-function-with-the-nuclio-cli-nuctl).
    To install the CLI, simply [download](https://github.com/nuclio/nuclio/releases) the appropriate CLI version to your installation machine.

<a id="run-nuclio"></a>
## Run Nuclio

Execute the following command from a command-line shell to run Nuclio on Docker.
> **Note:** The `stable` tag refers to the latest version released from the `master` branch of the Nuclio repository (unlike versioned branches, such as `1.3.x`).
```sh
docker run \
  --rm \
  --detach \
  --publish 8070:8070 \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --volume /tmp:/tmp \
  --name nuclio-dashboard \
  quay.io/nuclio/dashboard:stable-amd64
```

## What's next?

See the following resources to make the best of your new Nuclio environment:

- [Deploying Functions](/docs/tasks/deploying-functions.md)
- [More function examples](/hack/examples/README.md)
- [References](/docs/reference)
- [Best Practices and Common Pitfalls](/docs/concepts/best-practices-and-common-pitfalls.md)
