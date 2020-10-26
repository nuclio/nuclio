# Getting Started with Nuclio on Docker

Follow this step-by-step guide to set up a Nuclio development environment that uses Docker.

#### In this document

- [Prerequisites](#prerequisites)
- [Run Nuclio](#run-nuclio)
- [Deploy a function with the Nuclio dashboard](#deploy-a-function-with-the-nuclio-dashboard)
- [Deploy a function with the Nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli)
- [What's next](#whats-next)

## Prerequisites

Before starting the set-up procedure, ensure that the following prerequisites are met:

- Your environment has a running [Docker](https://docker.com) daemon.
    To ensure that your Docker daemon is running properly, run the following command with the same user that will be used to execute Nuclio CLI commands:
    ```sh
    docker version
    ```

- The [Nuclio CLI](/docs/reference/nuctl/nuctl.md) (`nuctl`) is installed &mdash; if you wish to [use the CLI to deploy Nuclio functions](#deploy-a-function-with-the-nuclio-cli).
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

<a id="deploy-a-function-with-the-nuclio-dashboard"></a>
## Deploy a function with the Nuclio dashboard

Browse to `http://localhost:8070` to see the [Nuclio dashboard](/README.md#dashboard).
Select the "default" project and then select **New Function** from the action toolbar to display the **Create function** page (http://localhost:8070/projects/default/create-function).
Choose one of the predefined template functions, and select **Deploy**. 
The first build populates the local Docker cache with base images and other files, so it might take a while to complete, depending on your network.
When the function deployment completes, you can select **Test** to invoke the function with a body.

<a id="deploy-a-function-with-the-nuclio-cli"></a>
## Deploy a function with the Nuclio CLI (nuctl)

Run the following Nuclio CLI (`nuctl`) command from a command-line shell to deploy the example [`helloworld`](/hack/examples/golang/helloworld/helloworld.go) Go function.
You can add the `--verbose` flag if you want to peek under the hood.
```sh
nuctl deploy helloworld \
    --path https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go
```

When the function deployment completes, you can get the function information by running the following CLI command:
```sh
nuctl get function helloworld
```
Sample output -
```sh
  NAMESPACE | NAME        | PROJECT | STATE | NODE PORT | REPLICAS  
  nuclio    | helloworld  | default | ready |     42089 | 1/1   
```
You can see from the sample output that the deployed function `helloworld` is running and using port `42089`.

Run the following CLI command to invoke the function:
```sh
nuctl invoke helloworld --method POST --body '{"hello":"world"}' --content-type "application/json"
```
Sample output -
```sh
> Response headers:
Server = nuclio
Date = Thu, 18 Jun 2020 06:56:27 GMT
Content-Type = application/text
Content-Length = 21

> Response body:
Hello, from Nuclio :]
```

## What's next?

See the following resources to make the best of your new Nuclio environment:

- [Deploying Functions](/docs/tasks/deploying-functions.md)
- [More function examples](/hack/examples/README.md)
- [References](/docs/reference)
- [Best Practices and Common Pitfalls](/docs/concepts/best-practices-and-common-pitfalls.md)

