# Getting Started with nuclio on Kubernetes

Follow this step-by-step guide to set up a nuclio development environment that uses a Kubernetes cluster.

#### In this document

- [Prerequisites](#prerequisites)
- [Install nuclio](#install-nuclio)
- [Deploy a function with the nuclio dashboard](#deploy-a-function-with-the-nuclio-dashboard)
- [Deploy a function with the nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli-nuctl)
- [What's next](#whats-next)

## Prerequisites

Before beginning with the installation, ensure that you have a [Kubernetes](https://kubernetes.io) **v1.7 or later** cluster and the credentials of a Docker registry, such as [Docker Hub](https://hub.docker.com/), [Azure Container Registry (ACR)](https://azure.microsoft.com/services/container-registry/), or [Google Container Registry (GCR)](https://cloud.google.com/container-registry/).

## Install nuclio

At this stage you should have a functioning Kubernetes cluster, a Docker registry, and a working Kubernetes CLI (`kubectl`), and you can proceed to install the nuclio services on the cluster (i.e., deploy nuclio). For more information about `kubectl`, see the [Kubernetes documentation](https://kubernetes.io/docs/user-guide/kubectl-overview/).

**Create a nuclio namespace** by running the following command:

> Note: All nuclio resources go into the "nuclio" namespace, and role-based access control (RBAC) is configured accordingly.

```sh
kubectl create namespace nuclio
```

**Create a registry secret:** because nuclio functions are images that need to be pushed and pulled to/from the registry, you need to create a secret that stores your registry credentials. Replace the `<...>` placeholders in the following commands with your username, password, and URL:
> Note: If you want to use Docker Hub, the URL is `registry.hub.docker.com`.

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

**Create the RBAC roles** that are required for using nuclio:
> Note: You are encouraged to look at the [**nuclio-rbac.yaml**](https://github.com/nuclio/nuclio/blob/master/hack/k8s/resources/nuclio-rbac.yaml) file that's used in the following command before applying it, so that you don't get into the habit of blindly running things on your cluster (akin to running scripts off the internet as root).

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio-rbac.yaml
```

**Deploy nuclio to the cluster:** the following command deploys the nuclio controller and dashboard, among other resources:

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio.yaml
```

> Note: In this example, the nuclio dashboard service has full access to the local machine's Docker daemon. If you're concerned about the security implications, isolate the dashboard in its own node. The nuclio team is working with the community to establish a secure and robust on-cluster build mechanism.

Use the command `kubectl get pods --namespace nuclio` to verify both the controller and dashboard are running.

**Forward the nuclio dashboard port:** the nuclio dashboard publishes a service at port 8070. To use the dashboard, you first need to forward this port to your local IP address:
```sh
kubectl port-forward -n nuclio $(kubectl get pods -n nuclio -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070
```

## Deploy a function with the nuclio dashboard

Browse to `http://localhost:8070` (after having forwarded this port as part of the nuclio installation). You should see the [nuclio dashboard](/README.md#dashboard) UI. Choose one of the built-in examples and click **Deploy**. The first build will populate the local Docker cache with base images and other files, so it might take a while, depending on your network. When the function deployment is completed, you can click **Invoke** to invoke the function with a body.

## Deploy a function with the nuclio CLI (nuctl)

Start by [downloading](https://github.com/nuclio/nuclio/releases) the latest version of the [`nuctl`](/docs/reference/nuctl/nuctl.md) nuclio CLI for your platform, and then deploy the `helloworld` Go sample function. You can add the `--verbose` flag if you want to peek under the hood:
> Note: If you are using Docker Hub, the URL here includes your username - `registry.hub.docker.com/<username>`.

```sh
nuctl deploy helloworld -n nuclio -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry <URL>
```

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

