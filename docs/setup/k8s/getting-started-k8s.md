# Getting Started with nuclio on Kubernetes

Follow this step-by-step guide to set up a nuclio development environment that uses a Kubernetes cluster.

#### In this document

- [Prepare Kubernetes](#prepare-kubernetes)
- [Install nuclio](#install-nuclio)
- [Deploy a function with the nuclio playground](#deploy-a-function-with-the-nuclio-playground)
- [Deploy a function with the nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli-nuctl)

## Prepare Kubernetes

To start deploying functions, you need a [Kubernetes](https://kubernetes.io) **v1.7 or later** cluster and access to a Docker registry.

### Minikube
If you're just getting started with Kubernetes, we recommend following our [Minikube installation guide](/docs/setup/k8s/install/k8s-install-minikube.md) before continuing. It will walk you through installing a Kubernetes cluster on a local VM with a built in Docker registry.

Before Docker images can be pushed to your built-in registry, you need to add your integrated Docker registry address to the list of insecure registries. For example, if you are using Minikube, you might add `$(minikube ip):5000`. If you are using Docker for Mac OS, you can find the IP address under **Preferences > Daemon**.

### Managed Kubernetes
If you're using a managed Kubernetes cluster like [GKE](/docs/setup/gke/getting-started-gke.md) or AKS (coming soon), head on over to the specific guide for that platform, including leveraging the private Docker registries. 

### Self hosted
If you already have a Kubernetes v1.7+ cluster, you just need to make sure you have access to some Docker registry. If you'd like to use the docker hub, specify `--registry <your account name>` in `nuctl deploy` (omitting `--run-registry`). Otherwise, specify the address of the private Docker registry (e.g. `--registry 10.0.0.1:8989`). The docker daemon must be authenticated to this registry on the machine running `nuctl`.

To use the `nuclio` playground, follow [GKE's playground section](/docs/setup/gke/getting-started-gke.md#deploy-a-function-with-the-nuclio-playground) on how to inject Docker credentials into the `nuclio` playground (documentation about this is coming soon).

> **Note:** For simplicity, this guide assumes that you are using Minikube. If you select to use another method, replace `$(minikube ip)` references in the commands with your cluster IP.

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

Start by downloading the latest [nuctl](https://github.com/nuclio/nuclio/releases) for your platform. 

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

