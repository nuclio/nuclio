# Getting Started with Nuclio on Minikube

Follow this step-by-step guide to set up Nuclio on [Minikube](https://github.com/kubernetes/minikube/), which is a single-node Kubernetes cluster that runs in a virtual machine (VM) on a local computer.

#### In this document

- [Prerequisites](#prerequisites)
- [Prepare Minikube](#prepare-minikube)
- [Install Nuclio](#install-nuclio)
- [Deploy a function with the Nuclio dashboard](#deploy-a-function-with-the-nuclio-dashboard)
- [Deploy a function with the Nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli)
- [What's next](#whats-next)

## Prerequisites

Before starting the set-up procedure, ensure that the following prerequisites are met:

- [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/) is installed on your installation machine.
    It's recommended that you use these drivers:

    - For **Mac OS** - [hyperkit driver](https://github.com/kubernetes/minikube/blob/master/docs/drivers.md#hyperkit-driver)
    - For **Linux** - [virtualbox](https://www.virtualbox.org/wiki/Linux_Downloads)

- The [Nuclio CLI](/docs/reference/nuctl/nuctl.md) (`nuctl`) is installed: simply [download](https://github.com/nuclio/nuclio/releases) the appropriate CLI version to your installation machine.

## Prepare Minikube

**Start Minikube** as you normally would. Note that the following command also enables role-based access control (RBAC) (which is disabled by default on Minikube version 0.24.1 and later) so that you can get more comfortable working with an RBAC-enabled Kubernetes cluster:

- **Mac OS**

    ```sh
    minikube start --vm-driver=hyperkit --extra-config=apiserver.authorization-mode=RBAC
    ```
- **Linux**

    ```sh
    minikube start --vm-driver=virtualbox --extra-config=apiserver.authorization-mode=RBAC
    ```

**Set admin permissions:** bypass Minikube configuration issues by giving cluster-admin permissions to the Kubernetes services, so that services such as `kube-dns` can work in Minikube:
> **Note:** You are encouraged to look at the [**kubedns-rbac**](https://github.com/nuclio/nuclio/blob/master/hack/minikube/resources/kubedns-rbac.yaml) file that's used in the following command, and the RBAC configuration files used in the [Nuclio installation](#install-nuclio) section, before applying the files, so that you don't get into the habit of blindly running things on your cluster (akin to running scripts off the internet as root).<br/>
> If you don't want to elevate your Kubernetes services, run Minikube with RBAC disabled (omit `--extra-config` from `minikube start`) and skip the RBAC related commands in the Nuclio installation instructions.

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/minikube/resources/kubedns-rbac.yaml
```

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
kubectl create namespace nuclio
```

**Create the RBAC roles** that are required for using Nuclio (provided you didn't disable RBAC when [preparing Minikube](#prepare-minikube)):
> **Note:** As indicated in the [Minikube preparation](#prepare-minikube) instructions, you are encouraged to look at the [**nuclio-rbac.yaml**](https://github.com/nuclio/nuclio/blob/master/hack/k8s/resources/nuclio-rbac.yaml) file that's used in the following command before applying it.

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio-rbac.yaml
```

**Deploy Nuclio to the cluster:** the following command deploys the Nuclio controller and dashboard, among other resources:

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio.yaml
```

Use the command `kubectl get pods --namespace nuclio` to verify both the controller and dashboard are running.

**Forward the Nuclio dashboard port:** the Nuclio dashboard publishes a service at port 8070. To use the dashboard, you first need to forward this port to your local IP address:
```sh
kubectl port-forward -n nuclio $(kubectl get pods -n nuclio -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070
```

<a id="deploy-a-function-with-the-nuclio-dashboard"></a>
## Deploy a function with the Nuclio dashboard

Browse to `http://localhost:8070` (after having forwarded this port as part of the Nuclio installation) to see the [Nuclio dashboard](/README.md#dashboard).
Select the "default" project and then select **New Function** from the action toolbar to display the **Create function** page (http://localhost:8070/projects/default/create-function).
Choose one of the predefined template functions, and select **Deploy**. 
The first build populates the local Docker cache with base images and other files, so it might take a while to complete, depending on your network.
When the function deployment completes, you can select **Invoke** to invoke the function with a body.

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

