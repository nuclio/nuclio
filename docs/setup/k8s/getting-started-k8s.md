# Getting Started with Nuclio on Kubernetes

Follow this step-by-step guide to set up a Nuclio development environment that uses a Kubernetes cluster.

#### In this document

- [Prerequisites](#prerequisites)
- [Install Nuclio](#install-nuclio)
- [Deploy a function with the Nuclio dashboard](#deploy-a-function-with-the-nuclio-dashboard)
- [Deploy a function with the Nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli)
- [What's next](#whats-next)

## Prerequisites

Before starting the set-up procedure, ensure that the following prerequisites are met:

- Your environment has a [Kubernetes](https://kubernetes.io) **v1.7 or later** cluster.

- You have the credentials of a Docker registry, such as [Docker Hub](https://hub.docker.com/), [Azure Container Registry (ACR)](https://azure.microsoft.com/services/container-registry/), or [Google Container Registry (GCR)](https://cloud.google.com/container-registry/).

- The [Nuclio CLI](/docs/reference/nuctl/nuctl.md) (`nuctl`) is installed &mdash; if you wish to [use the CLI to deploy Nuclio functions](#deploy-a-function-with-the-nuclio-cli).
    To install the CLI, simply [download](https://github.com/nuclio/nuclio/releases) the appropriate CLI version to your installation machine.

## Install Nuclio

At this stage you should have a functioning Kubernetes cluster, a Docker registry, and a working Kubernetes CLI (`kubectl`), and you can proceed to install the Nuclio services on the cluster (i.e., deploy Nuclio). For more information about `kubectl`, see the [Kubernetes documentation](https://kubernetes.io/docs/user-guide/kubectl-overview/).

**Create a Nuclio namespace** by running the following command:

> **Note:** All Nuclio resources go into the "nuclio" namespace, and role-based access control (RBAC) is configured accordingly.

```sh
kubectl create namespace nuclio
```

**Create a registry secret:** because Nuclio functions are images that need to be pushed and pulled to/from the registry, you need to create a secret that stores your registry credentials.
Replace the `<...>` placeholders in the following commands with your username, password, and URL:
> **Note:** If you want to use Docker Hub, the URL is `registry.hub.docker.com`.

```sh
read -s mypassword
<enter your password>

kubectl create secret docker-registry registry-credentials --namespace nuclio \
    --docker-username <username> \
    --docker-password $mypassword \
    --docker-server <URL> \
    --docker-email ignored@nuclio.io

unset mypassword
```

**Create the RBAC roles** that are required for using Nuclio:
> **Note:** You are encouraged to look at the [**nuclio-rbac.yaml**](https://github.com/nuclio/nuclio/blob/master/hack/k8s/resources/nuclio-rbac.yaml) file that's used in the following command before applying it, so that you don't get into the habit of blindly running things on your cluster (akin to running scripts off the internet as root).

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio-rbac.yaml
```

**Deploy Nuclio to the cluster:** the following command deploys the Nuclio controller and dashboard, among other resources:

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio.yaml
```

> **Note:** In this example, the Nuclio dashboard service has full access to the local machine's Docker daemon. If you're concerned about the security implications, isolate the dashboard in its own node. The Nuclio team is working with the community to establish a secure and robust on-cluster build mechanism.

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
When the function deployment completes, you can select **Test** to invoke the function with a body.

<a id="deploy-a-function-with-the-nuclio-cli"></a>
## Deploy a function with the Nuclio CLI (nuctl)

Run the following Nuclio CLI (`nuctl`) command from a command-line shell to deploy the example [`helloworld`](/hack/examples/golang/helloworld/helloworld.go) Go function.
If you're using Docker Hub, the URL should include your username - `docker.io/<username>` - and you might also need to log into your Docker Hub account (`docker login`) on the installation machine before running the deployment command.
You can add the `--verbose` flag if you want to peek under the hood.
```sh
nuctl deploy helloworld -n nuclio -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry <URL>
```

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
- [Running Nuclio in a production environment over Kubernetes](/docs/setup/k8s/running-in-production-k8s.md)
- [References](/docs/reference)
- [Best Practices and Common Pitfalls](/docs/concepts/best-practices-and-common-pitfalls.md)

