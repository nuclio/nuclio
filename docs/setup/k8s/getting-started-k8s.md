# Installing Nuclio on Kubernetes

Follow this step-by-step guide to set up a Nuclio development environment that uses a Kubernetes cluster.

#### In this document

- [Prerequisites](#prerequisites)
- [Install Nuclio](#install-nuclio)
- [What's next](#what-s-next)

## Prerequisites

Before starting the set-up procedure, ensure that the following prerequisites are met:

- Your environment has a [Kubernetes](https://kubernetes.io) **v1.19 or later** cluster.

- You have the credentials of a Docker registry, such as [Docker Hub](https://hub.docker.com/), [Azure Container Registry (ACR)](https://azure.microsoft.com/services/container-registry/), or [Google Container Registry (GCR)](https://cloud.google.com/container-registry/).

- The [Nuclio CLI](../../reference/nuctl/nuctl.md) (`nuctl`) is installed &mdash; if you wish to [use the CLI to deploy Nuclio functions](#deploy-a-function-with-the-nuclio-cli).
    To install the CLI, simply [download](https://github.com/nuclio/nuclio/releases) the appropriate CLI version to your installation machine.

## Install Nuclio

At this stage you should have a functioning Kubernetes cluster, credentials to a private Docker registry, and a working Kubernetes CLI (`kubectl`), and you can proceed to install the Nuclio services on the cluster (i.e., deploy Nuclio). For more information about `kubectl`, see the [Kubernetes documentation](https://kubernetes.io/docs/reference/kubectl/).

Follow the instructions of [How to run nuclio in Production](running-in-production-k8s.md)


Use the command `kubectl --namespace nuclio get pods` to verify both the controller and dashboard are running.

## What's next?

See the following resources to make the best of your new Nuclio environment:

- [Deploying Functions](../../tasks/deploying-functions.md)
- [Invoking Functions by Name with a Kubernetes Ingress](../../concepts/k8s/function-ingress.md)
- [More function examples](../../examples/README.md)
- [References](../../reference/index)
- [Best Practices and Common Pitfalls](../../concepts/best-practices-and-common-pitfalls.md)
