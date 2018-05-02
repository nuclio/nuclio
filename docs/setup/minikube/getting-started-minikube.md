# Getting Started with nuclio on Minikube

Follow this step-by-step guide to set up nuclio on [Minikube](https://github.com/kubernetes/minikube/), which is a single-node Kubernetes cluster that runs in a virtual machine (VM) on a local computer.

#### In this document

- [Prerequisites](#prerequisites)
- [Prepare Minikube](#prepare-minikube)
- [Install nuclio](#install-nuclio)
- [Deploy a function with the nuclio dashboard](#deploy-a-function-with-the-nuclio-dashboard)
- [Deploy a function with the nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli-nuctl)
- [What's next](#whats-next)

## Prerequisites

Ensure that the following components are installed on your installation machine:

- [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)
- [xhyve driver](https://github.com/kubernetes/minikube/blob/master/docs/drivers.md#xhyve-driver)

## Prepare Minikube

**Start Minikube** as you normally would. Note that the following command also enables role-based access control (RBAC) (which is disabled by default on Minikube version 0.24.1 and later) so that you can get more comfortable working with an RBAC-enabled Kubernetes cluster:

```sh
minikube start --vm-driver=xhyve --extra-config=apiserver.Authorization.Mode=RBAC
```

**Set admin permissions:** bypass Minikube configuration issues by giving cluster-admin permissions to the Kubernetes services, so that services such as `kube-dns` can work in Minikube:
> Note: You are encouraged to look at the [**kubedns-rbac**](https://github.com/nuclio/nuclio/blob/master/hack/minikube/resources/kubedns-rbac.yaml) file that's used in the following command, and the RBAC configuration files used in the [nuclio installation](#install-nuclio) section, before applying the files, so that you don't get into the habit of blindly running things on your cluster (akin to running scripts off the internet as root).<br/>
> If you don't want to elevate your Kubernetes services, run Minikube with RBAC disabled (omit `--extra-config` from `minikube start`) and skip the RBAC related commands in the nuclio installation instructions.

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/minikube/resources/kubedns-rbac.yaml
```

**Bring up a Docker registry inside Minikube.** You'll later push your functions to this registry:

> Note: You can skip this step if you're a more advanced user and would like to use another type of registry, such as [Docker Hub](https://hub.docker.com/), [Azure Container Registry (ACR)](https://azure.microsoft.com/services/container-registry/), or [Google Container Registry (GCR)](https://cloud.google.com/container-registry/). See [Getting started with Kubernetes](/docs/setup/k8s/getting-started-k8s.md) for instructions. 

```sh
minikube ssh -- docker run -d -p 5000:5000 registry:2
```

Before Docker images can be pushed to your built-in registry, you need to add its address (`$(minikube ip):5000`) to the list of insecure registries. If you're using Docker for Mac OS, you can add it under **Preferences > Daemon**.

## Install nuclio

At this stage you should have a functioning Kubernetes cluster, a Docker registry, and a working Kubernetes CLI (`kubectl`), and you can proceed to install the nuclio services on the cluster (i.e., deploy nuclio).

**Create a nuclio namespace** by running the following command:

> Note: All nuclio resources go into the "nuclio" namespace, and role-based access control (RBAC) is configured accordingly.

```sh
kubectl create namespace nuclio
```

**Create the RBAC roles** that are required for using nuclio (provided you didn't disable RBAC when [preparing Minikube](#prepare-minikube)):
> As indicated in the [Minikube preparation](#prepare-minikube) instructions, you are encouraged to look at the [**nuclio-rbac.yaml**](https://github.com/nuclio/nuclio/blob/master/hack/k8s/resources/nuclio-rbac.yaml) file that's used in the following command before applying it.

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio-rbac.yaml
```

**Deploy nuclio to the cluster:** the following command deploys the nuclio controller and dashboard, among other resources:

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio.yaml
```

Use the command `kubectl get pods --namespace nuclio` to verify both the controller and dashboard are running.

**Forward the nuclio dashboard port:** the nuclio dashboard publishes a service at port 8070. To use the dashboard, you first need to forward this port to your local IP address:
```sh
kubectl port-forward -n nuclio $(kubectl get pods -n nuclio -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070
```

## Deploy a function with the nuclio dashboard

Browse to `http://localhost:8070` (after having forwarded this port as part of the nuclio installation). You should see the [nuclio dashboard](/README.md#dashboard) UI. Choose one of the built-in examples and click **Deploy**. The first build will populate the local Docker cache with base images and other files, so it might take a while, depending on your network. When the function deployment is completed, you can click **Invoke** to invoke the function with a body.

## Deploy a function with the nuclio CLI (nuctl)

Start by [downloading](https://github.com/nuclio/nuclio/releases) the latest version of the [`nuctl`](/docs/reference/nuctl/nuctl.md) nuclio CLI for your platform, and then deploy the `helloworld` Go sample function. You can add the `--verbose` flag if you want to peek under the hood:

```sh
nuctl deploy helloworld -n nuclio -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry $(minikube ip):5000 --run-registry localhost:5000
```

> Note: The difference between the two registries specified in this command and the reason for their addresses being different is as follows:
>
> - The `--registry` option defines the Docker registry onto which the function images that you build will be pushed. This is the registry that you previously brought up on your Minikube VM.
> - The `--registry-run` option defines the registry from which the [`kubelet`](https://kubernetes.io/docs/reference/generated/kubelet/) Kubernetes "node agent" will pull the function images. Because this operation occurs in the Minikube VM, the command specifies `localhost` instead of the VM's IP address.

Then, invoke the function:

```sh
nuctl invoke -n nuclio helloworld
```

## What's next?

See the following resources to make the best of your new nuclio environment:

- [Deploying functions](/docs/tasks/deploying-functions.md)
- [Invoking functions by name with an ingress](/docs/concepts/k8s/function-ingress.md)
- [More function examples](/hack/examples/README.md)
- [References](/docs/reference/)

