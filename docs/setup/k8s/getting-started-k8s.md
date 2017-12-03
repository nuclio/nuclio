# Getting Started with nuclio on Kubernetes

Follow this step-by-step guide to set up a nuclio development environment that uses a Kubernetes cluster.

#### In this document

- [Install Kubernetes](#install-kubernetes)
- [Install nuclio](#install-nuclio)
- [Deploy a function with the nuclio playground](#deploy-a-function-with-the-nuclio-playground)
- [Deploy a function with the nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli-nuctl)

## Install Kubernetes

To start deploying functions, you need a [Kubernetes](https://kubernetes.io) **v1.7 or later** cluster; nuclio uses Custom Resource Definitions (CRDs), which were introduced in Kubernetes v1.7. You can prepare the cluster in one of three ways:

1. [Using Minikube on a local virtual machine (VM)](/docs/setup/k8s/install/k8s-install-minikube.md).
   This method is recommended for beginners.
2. [From scratch, using kubeadm on Linux Ubuntu](/docs/setup/k8s/install/k8s-install-kubeadm-linux.md).
3. [On an existing Kubernetes cluster](/docs/setup/k8s/install/k8s-install-w-existing-cluster.md).

> **Note:** For simplicity, this guide assumes that you are using Minikube. If you select to use another method, simply replace `$(minikube ip)` references in the commands with your cluster IP.

## Install nuclio

After following your selected Kubernetes installation instructions, you should have a functioning Kubernetes cluster, a Docker registry, and a working local Kubernetes CLI (`kubectl`). Now, you can go ahead and install the nuclio services on the cluster:

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/controller.yaml
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/playground.yaml
```

Use the command `kubectl get pods` to verify that both the controller and playground have a status of `Running`. For more information about `kubectl`, see the [Kubernetes documentation](https://kubernetes.io/docs/user-guide/kubectl-overview/).

## Deploy a function with the nuclio playground

Browse to `http://$(minikube ip):32050`.
You should be greeted by the [nuclio playground](/README.md#playground). Choose one of the built-in examples, and click **Deploy**. The first build will populate the local Docker cache with base images and other files, so it might take a while, depending on your network. When the function deployment is completed, you can click **Invoke** to invoke the function with a body.

## Deploy a function with the nuclio CLI (nuctl)

<a id="go-supported-version"></a>First, ensure that you have v1.8 or later of the Go (Golang) programming language (see https://golang.org/doc/install), and Docker (see https://docs.docker.com/engine/installation). Then, create a Go workspace (for example, in **~/nuclio**):

```sh
export GOPATH=~/nuclio && mkdir -p $GOPATH
```

Now, build [`nuctl`](/docs/reference/nuctl/nuctl.md), the nuclio command-line tool (CLI), and add `$GOPATH/bin` to the path for this session:

```sh
go get -u github.com/nuclio/nuclio/cmd/nuctl
PATH=$PATH:$GOPATH/bin
```

Before Docker images can be pushed to your built-in registry, you need to add your integrated Docker registry address to the list of insecure registries. For example, if you are using Minikube, you might add `$(minikube ip):5000`. If you are using Docker for Mac OS, you can find the IP address under **Preferences > Daemon**.

Deploy the `helloworld` Go sample function; you can add the `--verbose` flag if you want to peek under the hood:

```sh
nuctl deploy -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry $(minikube ip):5000 helloworld --run-registry localhost:5000
```

And finally, execute the function:

```sh
nuctl invoke helloworld
```

## What's next?

See the following resources to make the best of your new nuclio environment:

1. [Configuring a function](/docs/concepts/configuring-a-function.md)
2. [Invoking functions by name with an ingress](/docs/concepts/k8s/function-ingress.md)
3. [More function examples](/hack/examples/README.md)

