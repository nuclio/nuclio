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

Start Minikube as you normally would. Note that we are enabling RBAC (disabled by default, as of Minikube 0.24.1) so that we can get more comfortable working with an RBAC enabled Kubernetes cluster:

```sh
minikube start --vm-driver=xhyve --extra-config=apiserver.Authorization.Mode=RBAC
```

Work around configuration issues in Minikube by giving the kube services cluster admin so that things like kube-dns work in Minikube:

> Note: You are encouraged to peek at the file before applying it, so that you don't get into the habit of blindly running things on your cluster (akin to running scripts off the internet as root).
> If you don't want to elevate your kube services, run Minikube with RBAC disabled (omit `--extra-config` from `minikube start`) and skip applying RBAC related files over the course of installing nuclio

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/minikube/resources/kubedns-rbac.yaml
```

Bring up a docker registry inside Minikube. You'll later push your functions to this registry:

> Note: You can skip this step if you're a more advanced user and would like to use some other registry like the Docker hub, GCR, ACR, etc. See [Getting started with Kubernetes](/docs/setup/k8s/getting-started-k8s.md) for instructions. 

```sh
minikube ssh -- docker run -d -p 5000:5000 registry:2
```

## Install nuclio

After following your selected Kubernetes installation instructions, you should have a functioning Kubernetes cluster, a Docker registry, and a working local Kubernetes CLI (`kubectl`). Let's create the nuclio namespace, where all the services and deployed functions will go. 
> Note: It is possible to create complex multi tenant setups. Check out TODO to read more on how to go about doing that  

```sh
kubectl create namespace nuclio
```

Now you can go ahead and install the nuclio services and RBAC rules on the cluster:

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio.yaml
```

Use the command `kubectl get pods --namespace nuclio` to verify that both the controller and playground have a status of `Running`. For more information about `kubectl`, see the [Kubernetes documentation](https://kubernetes.io/docs/user-guide/kubectl-overview/).

## Deploy a function with the nuclio playground

Browse to the equivalent of `http://$(minikube ip):32050`. You should be greeted by the [nuclio playground](/README.md#playground). Choose one of the built-in examples, and click **Deploy**. The first build will populate the local Docker cache with base images and other files, so it might take a while, depending on your network. When the function deployment is completed, you can click **Invoke** to invoke the function with a body.

## Deploy a function with the nuclio CLI (nuctl)

Start by downloading the latest [nuctl](https://github.com/nuclio/nuclio/releases) for your platform. 

Deploy the `helloworld` Go sample function; you can add the `--verbose` flag if you want to peek under the hood:

```sh
nuctl deploy -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry $(minikube ip):5000 helloworld --run-registry localhost:5000
```

And finally, invoke the function:

```sh
nuctl invoke helloworld
```

## What's next?

See the following resources to make the best of your new nuclio environment:

1. [Configuring a function](/docs/concepts/configuring-a-function.md)
2. [Invoking functions by name with an ingress](/docs/concepts/k8s/function-ingress.md)
3. [More function examples](/hack/examples/README.md)
