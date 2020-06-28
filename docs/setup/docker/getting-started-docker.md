# Getting Started with Nuclio on Docker

Follow this step-by-step guide to set up a Nuclio development environment that uses Docker

#### In this document

- [Prerequisites](#prerequisites)
- [Run Nuclio](#run-nuclio)
- [Deploy a function with the Nuclio dashboard](#deploy-a-function-with-the-nuclio-dashboard)
- [Deploy a function with the Nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli-nuctl)
- [What's next](#whats-next)

## Prerequisites

Before beginning with the installation, ensure that you have a [Docker](https://docker.com) daemon running.

To ensure your Docker daemon is running properly, run  `docker version` with the same user that will execute `nuctl` commands.

## Run Nuclio

To run nuclio on Docker, simply run the following command:

```bash
docker run \
  --rm \
  --detach \
  --publish 8070:8070 \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --volume /tmp:/tmp \
  --name nuclio-dashboard \
  quay.io/nuclio/dashboard:stable-amd64
```

> NOTE: _stable_ tag refers to the latest version released by nuclio from `master` branch (unlike versioned branches, e.g.: 1.3.x)

Browse to `http://localhost:8070` to explore Nuclio

## Deploy a function with the Nuclio dashboard

Browse to `http://localhost:8070/projects/default/create-function`.
You should see the [Nuclio dashboard](/README.md#dashboard) UI.
Choose one of the built-in examples and click **Deploy**. 
The first build will populate the local Docker cache with base images and other files, so it might take a while, depending on your network.
When the function deployment is completed, you can click **Invoke** to invoke the function with a body.

## Deploy a function with the Nuclio CLI (nuctl)

Deploy an _HelloWorld_ example function by executing the following command:

```bash
nuctl deploy helloworld \
    --path https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go
```

Once function deployment is completed, You can get the function information by running:

```bash
nuctl get function helloworld
```

An example output:
```bash
  NAMESPACE | NAME        | PROJECT | STATE | NODE PORT | REPLICAS  
  nuclio    | helloworld  | default | ready |     42089 | 1/1   
``` 

You can see from the example output that the deployed function `helloworld` is _running_ and using port `42089`.

To invoke the function, run

```bash
nuctl invoke helloworld --method POST --body '{"hello":"world"}' --content-type "application/json"
```

An example output:

```bash
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

- [Best Practices and Common Pitfalls](/docs/concepts/best-practices-and-common-pitfalls.md)
- [Deploying functions](/docs/tasks/deploying-functions.md)
- [More function examples](/hack/examples/README.md)
- [References](/docs/reference)
