# Getting Started with nuclio on Kubernetes

#### In This Document

- [Overview](#overview)
- [Deploying a Function with the nuclio Playground](#deploying-a-function-with-the-nuclio-playground)
- [Deploying a Function with nuctl, the nuclio Command-Line Tool](#deploying-a-function-with-nuctl-the-nuclio-command-line-tool)

## Overview

To start deploying functions, you need a remote Kubernetes **v1.7+** cluster; (nuclio uses CRDs, which were introduced in Kubernetes v1.7). You can prepare the cluster in one of three ways:

1. [Using Minikube on a local virtual machine (VM)](install/minikube.md).
   This method is recommended for beginners.
2. [From scratch, using kubeadm on Ubuntu](install/linux.md).
3. [On an existing Kubernetes cluster](install/existing.md).

To keep things simple, this guide assumes that you are using Minikube. If you select to use another method, simply replace `$(minikube ip)` references in the commands with your cluster IP.

With a functioning Kubernetes cluster, a Docker registry, and a working local `kubectl` CLI, you can go ahead and install the nuclio services on the cluster:

```bash
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/controller.yaml
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/playground.yaml
```

Use the command `kubectl get pods` to verify that both the controller and playground have a status of `Running`. For more information about `kubectl`, see the [Kubernetes documentation](https://kubernetes.io/docs/user-guide/kubectl-overview/).

## Deploying a Function with the nuclio Playground

Browse to `http://$(minikube ip):32050`.
You should be greeted by the nuclio playground. Choose one of the built-in examples, and click **Deploy**. The first build will populate the local Docker cache with base images and other files, so it might take a while, depending on your network. When the function deployment is completed, you can click **Invoke** to invoke the function with a body.

## Deploying a Function with nuctl, the nuclio Command-Line Tool

First, ensure that you have Go (Golang) v1.8+ (https://golang.org/doc/install) and Docker (https://docs.docker.com/engine/installation), and create a Go workspace (for example, in `~/nuclio`):

```bash
export GOPATH=~/nuclio && mkdir -p $GOPATH
```

Then, build [`nuctl`](/docs/nuctl/nuctl.md), the nuclio command-line tool (CLI), and add `$GOPATH/bin` to the path for this session:
```bash
go get -u github.com/nuclio/nuclio/cmd/nuctl
PATH=$PATH:$GOPATH/bin
```

Before Docker images can be pushed to your built-in registry, you need to add your integrated Docker registry address to the list of insecure registries. For example, if you are using Minikube, you might add `$(minikube ip):5000`. If you are using Docker for Mac OS, you can find the IP address under **Preferences > Daemon**.

Deploy the `helloworld` Go sample function; you can add the `--verbose` flag if you want to peek under the hood:
```bash
nuctl deploy -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry $(minikube ip):5000 helloworld --run-registry localhost:5000
```

And finally, execute the function:
```bash
nuctl invoke helloworld
```

## What's Next?

See the following resources to make the best of your new nuclio environment:

1. [Configuring a function](/docs/configuring-a-function.md)
2. [Invoking functions by name with an ingress](function-ingress.md)
3. [More function examples](/hack/examples/README.md)

