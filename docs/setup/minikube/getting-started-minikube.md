# Getting Started with nuclio on Minikube

Follow this step-by-step guide to set up nuclio on Minikube - a Kubernetes cluster on a VM.

#### In this document

- [Prepare Minikube](#prepare-minikube)
- [Install nuclio](#install-nuclio)
- [Deploy a function with the nuclio playground](#deploy-a-function-with-the-nuclio-playground)
- [Deploy a function with the nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli-nuctl)

## Prepare Minikube

Ensure that the following components are installed on your installation machine:

- [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)
- [xhyve driver](https://github.com/kubernetes/minikube/blob/master/docs/drivers.md#xhyve-driver)

Start Minikube as you normally would:

```sh
minikube start --vm-driver=xhyve
```

Bring up a docker registry inside Minikube. You'll later push your functions to this registry:

```sh
minikube ssh -- docker run -d -p 5000:5000 registry:2
```

> Note: You can skip this step if you're a more advanced user and would like to use some other registry like the Docker hub, GCR, ACR, etc. See the docker registry guide to set that up.

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

