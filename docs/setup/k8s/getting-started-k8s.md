# Getting Started with nuclio on Kubernetes

Follow this step-by-step guide to set up a nuclio development environment that uses a Kubernetes cluster.

#### In this document

- [Install nuclio](#install-nuclio)
- [Deploy a function with the nuclio playground](#deploy-a-function-with-the-nuclio-playground)
- [Deploy a function with the nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli-nuctl)

## Install nuclio

To start deploying functions, you need a [Kubernetes](https://kubernetes.io) **v1.7 or later** cluster and the credentials of a docker registry (this could be Docker hub, Azure Container Registry, Google Container Registry, etc.). 

Let's start by creating the nuclio namespace:

```sh
kubectl create namespace nuclio
```

Since nuclio functions are images, we will need to create a secret holding our credentials to a registry. Specify `username`, `password` and `URL`:
> Note: The Docker hub URL is `hub.docker.com/u/<username>`

```sh
kubectl create secret docker-registry registry-credentials \
    --docker-username <username> \
    --docker-password <username> \
    --docker-server <URL> \
    --docker-email ignored@nuclio.io
```

Now you can go ahead and install the nuclio services and RBAC rules on the cluster:

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio.yaml
```

Use the command `kubectl get pods --namespace nuclio` to verify that both the controller and playground have a status of `Running`. For more information about `kubectl`, see the [Kubernetes documentation](https://kubernetes.io/docs/user-guide/kubectl-overview/).








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

