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
> Note: If you want to use the Docker, the URL is `registry.hub.docker.com`

```sh
kubectl create secret docker-registry registry-credentials --namespace nuclio \
    --docker-username <username> \
    --docker-password <username> \
    --docker-server <URL> \
    --docker-email ignored@nuclio.io
```

Now you can go ahead and install the nuclio services and RBAC rules on the cluster:
> Note: You are encouraged to peek at the file before applying it, so that you don't get into the habit of blindly running things on your cluster (akin to running scripts off the internet as root). In the case below, the playground service has full access to the local machine's Docker daemon. If you are concerned about the security implications, isolate the playground in its own node. We are working with the community to establish a secure and robust on-cluster build mechanism

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio.yaml
```

Use the command `kubectl get pods --namespace nuclio` to verify that both the controller and playground have a status of `Running`. For more information about `kubectl`, see the [Kubernetes documentation](https://kubernetes.io/docs/user-guide/kubectl-overview/).

## Deploy a function with the nuclio playground

The playground publishes a service at port 8070. We will need to port forward this to our local IP address:

```sh
kubectl port-forward $(kubectl get po -l nuclio.io/app=playground -o jsonpath='{.items[0].metadata.name}') 8070:8070
```

And then browse to `http://localhost:8070`. You should be greeted by the [nuclio playground](/README.md#playground). Choose one of the built-in examples, and click **Deploy**. The first build will populate the local Docker cache with base images and other files, so it might take a while depending on your network. When the function deployment is completed, you can click **Invoke** to invoke the function with a body.

## Deploy a function with the nuclio CLI (nuctl)

Start by downloading the latest [nuctl](https://github.com/nuclio/nuclio/releases) for your platform and then deploy the `helloworld` Go sample function. You can add the `--verbose` flag if you want to peek under the hood:

```sh
nuctl deploy -n nuclio -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry $(minikube ip):5000 helloworld --run-registry localhost:5000
```

And finally, invoke the function:

```sh
nuctl invoke -n nuclio helloworld
```

## What's next?

See the following resources to make the best of your new nuclio environment:

1. [Configuring a function](/docs/concepts/configuring-a-function.md)
2. [Invoking functions by name with an ingress](/docs/concepts/k8s/function-ingress.md)
3. [More function examples](/hack/examples/README.md)
