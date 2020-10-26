# Getting Started with Nuclio on Minikube

Follow this step-by-step guide to set up Nuclio on [Minikube](https://github.com/kubernetes/minikube/), which is a tool that lets you run Kubernetes locally.

#### In this document

- [Prerequisites](#prerequisites)
- [Prepare Minikube](#prepare-minikube)
- [Install Nuclio](#install-nuclio)
- [Deploy a function with the Nuclio dashboard](#deploy-a-function-with-the-nuclio-dashboard)
- [Deploy a function with the Nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli)
- [What's next](#whats-next)

## Prerequisites

Before starting the set-up procedure, ensure that the following prerequisites are met:

- The following components are installed on your installation machine:
    - [Docker](https://docs.docker.com/get-docker/)
    - [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)
    - [Helm](https://helm.sh/docs/intro/install/)
- The [Nuclio CLI](/docs/reference/nuctl/nuctl.md) (`nuctl`) is installed &mdash; if you wish to [use the CLI to deploy Nuclio functions](#deploy-a-function-with-the-nuclio-cli).
    To install the CLI, simply [download](https://github.com/nuclio/nuclio/releases) the appropriate CLI version to your installation machine.
- It's recommended that you use these drivers:

    - For **Mac OS** - [Docker](https://minikube.sigs.k8s.io/docs/drivers/docker/)
    - For **Linux** - [Docker](https://minikube.sigs.k8s.io/docs/drivers/docker/) or [KVM2](https://minikube.sigs.k8s.io/docs/drivers/kvm2/)
    - For **Windows** - [Docker](https://minikube.sigs.k8s.io/docs/drivers/docker/) or [Hyper-V](https://minikube.sigs.k8s.io/docs/drivers/hyperv/)

## Prepare Minikube

**Start Minikube** as you normally would.
Note that the following command also enables role-based access control (RBAC) so that you can get more comfortable working with an RBAC-enabled Kubernetes cluster:

```sh
minikube start --kubernetes-version v1.17.9 --driver docker --extra-config=apiserver.authorization-mode=RBAC --addons ingress
```

> **Note:** You may want to
> - Change the Kubernetes version. Currently, the recommended version is 1.17.9.
> - Add `--addons ingress` to your `minikube start` command to support creating function ingresses.
>     Ensure that your function ingress appears on your hosts file (**/etc/hosts**).
>     You can do this by running this command:
>     ```sh
>     echo "$(minikube ip) my-function.info" | sudo tee -a /etc/hosts
>     ```

**Bring up a Docker registry inside Minikube.** You'll later push your functions to this registry:

> **Note:** You can skip this step if you're a more advanced user and would like to use another type of registry, such as [Docker Hub](https://hub.docker.com/), [Azure Container Registry (ACR)](https://azure.microsoft.com/services/container-registry/), or [Google Container Registry (GCR)](https://cloud.google.com/container-registry/). See [Getting started with Kubernetes](/docs/setup/k8s/getting-started-k8s.md) for instructions. 

```sh
minikube ssh -- docker run -d -p 5000:5000 registry:2
```

Before container images can be pushed to your built-in registry, you need to add its address (`$(minikube ip):5000`) to the list of insecure registries:

- **Docker for Mac OS** -  you can add it under **Preferences | Daemon**.
- **Linux** - follow the instructions in the [Docker documentation](https://docs.docker.com/registry/insecure/#deploy-a-plain-http-registry).

## Install Nuclio

At this stage you should have a functioning Kubernetes cluster, a Docker registry, and a working Kubernetes CLI (`kubectl`), and you can proceed to install the Nuclio services on the cluster (i.e., deploy Nuclio).

**Create a Nuclio namespace** by running the following command:

> **Note:** All Nuclio resources go into the "nuclio" namespace, and role-based access control (RBAC) is configured accordingly.

```sh
minikube kubectl -- create namespace nuclio
```

**Add nuclio to helm repo charts:** the following commands add Nuclio repo charts to your helm repos:
```sh
helm repo add nuclio https://nuclio.github.io/nuclio/charts
```

**Deploy Nuclio to the cluster:** the following command deploys Nuclio and its minimum required Kubernetes resources (including RBAC roles):

```sh
helm --namespace nuclio install nuclio nuclio/nuclio
```

Use the command `minikube kubectl -- get pods --namespace nuclio` to verify both the controller and dashboard are running.

**Forward the Nuclio dashboard port:** the Nuclio dashboard publishes a service at port 8070. To use the dashboard, you first need to forward this port to your local IP address:
```sh
minikube kubectl -- port-forward -n nuclio $(kubectl get pods -n nuclio -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070
```

<a id="deploy-a-function-with-the-nuclio-dashboard"></a>
## Deploy a function with the Nuclio dashboard

Browse to `http://localhost:8070` (after having forwarded this port as part of the Nuclio installation) to see the [Nuclio dashboard](/README.md#dashboard).
Select the "default" project and then select **New Function** from the action toolbar to display the **Create function** page (http://localhost:8070/projects/default/create-function).
Choose one of the predefined template functions, and select **Deploy**. 
The first build populates the local Docker cache with base images and other files, so it might take a while to complete, depending on your network.
When the function deployment completes, you can select **Test** to invoke the function with a body.

<a id="deploy-a-function-with-the-nuclio-cli"></a>
## Deploy a function with the Nuclio CLI (nuctl)

Run the following Nuclio CLI (`nuctl`) command from a command-line shell to deploy the example [`helloworld`](/hack/examples/golang/helloworld/helloworld.go) Go function.
You can add the `--verbose` flag if you want to peek under the hood.
```sh
nuctl deploy helloworld -n nuclio -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry $(minikube ip):5000 --run-registry localhost:5000
```
> **Note:** The difference between the two registries specified in this command and the reason for their addresses being different is as follows:
>
> - The `--registry` option defines the Docker registry onto which the function images that you build will be pushed. This is the registry that you previously brought up on your Minikube VM.
> - The `--registry-run` option defines the registry from which the [`kubelet`](https://kubernetes.io/docs/reference/generated/kubelet/) Kubernetes "node agent" will pull the function images. Because this operation occurs in the Minikube VM, the command specifies `localhost` instead of the VM's IP address.

When the function deployment completes, you can get the function information by running the following CLI command:
```sh
nuctl get function helloworld
```
Sample output -
```sh
  NAMESPACE | NAME        | PROJECT | STATE | NODE PORT | REPLICAS  
  nuclio    | helloworld  | default | ready |     42089 | 1/1   
```
You can see from the sample output that the deployed function `helloworld` is running and using port `42089`.

Run the following CLI command to invoke the function:
```sh
nuctl invoke helloworld --method POST --body '{"hello":"world"}' --content-type "application/json"
```
Sample output -
```sh
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

- [Deploying Functions](/docs/tasks/deploying-functions.md)
- [Invoking Functions by Name with a Kubernetes Ingress](/docs/concepts/k8s/function-ingress.md)
- [More function examples](/hack/examples/README.md)
- [References](/docs/reference/)
- [Best Practices and Common Pitfalls](/docs/concepts/best-practices-and-common-pitfalls.md)

