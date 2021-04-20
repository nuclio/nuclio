# Nuclio

##  High-Performance Serverless Event and Data Processing Platform

[Nuclio](https://nuclio.io) is a new "serverless" project, derived from [Iguazio](https://iguazio.com)'s elastic data life-cycle management service for high-performance events and data processing

## Introduction

This chart bootstraps a Nuclio deployment (controller and dashboard) and service on a Kubernetes cluster using the Helm Package manager. Before you get started you will need:

- A Kubernetes 1.15+ cluster
- helm (with tiller installed if using helm2, we recommend helm3)
- kubectl

### Adding Nuclio to helm
Until the Nuclio helm chart is accepted into the upstream repository, we must start by adding the Nuclio repository to helm:

```sh
helm repo add nuclio https://nuclio.github.io/nuclio/charts
```

## Installing Nuclio
The Nuclio helm chart allows a range of options that allow installation across different Kubernetes providers. The recommended method of creating a secret with your Docker registry credentials is presented below.

> Note: You can skip this if you're using Minikube with an insecure registry

Start by creating a namespace:
``` sh
kubectl create namespace nuclio
```

**Create a registry secret:** because Nuclio functions are images that need to be pushed and pulled to/from the registry, you need to create a secret that stores your registry credentials.
Replace the `<...>` placeholders in the following commands with your username, password, and URL:
> **Note:** If you want to use Docker Hub, the URL is `registry.hub.docker.com`.

Create the secret:
``` sh
read -s mypassword
<enter your password>

kubectl create secret docker-registry nuclio-registry-credentials \
    --docker-username <username> \
    --docker-password $mypassword \
    --docker-server <registry name> \
    --docker-email ignored@nuclio.io

unset mypassword
```

### Install on Minikube, vanilla Kubernetes, AKS
There are no special flags required when installing in AKS or vanilla Kubernetes:

``` sh
helm install nuclio nuclio/nuclio
```

### Install on GKE (or when using GCR)
If you're using GCR as your image registry, there is a small quirk where the login URL is different from the push/pull URL. By default, Nuclio will take the push/pull URL from the secret, but in this case we need to let Nuclio know what the push/pull URL is:

``` sh
helm install nuclio \
	--set registry.pushPullUrl gcr.io/<your project name> \
	nuclio/nuclio
```

### Install on Minikube using a local, insecure registry

You will need to run a local Docker registry. Run the following command on the host if you're working with Docker for Mac or on the Minikube VM:
```sh
docker run -d -p 5000:5000 registry:2
```

By not providing a registry secret name (`registry.secretName`) nor credentials (`registry.credentials.username` / `registry.credentials.password`), Nuclio will understand credentials are not needed, and not try to load Docker secrets.

``` sh
helm install nuclio \
    --set registry.pushPullUrl=localhost:5000 \
	nuclio/nuclio
```

Forward the dashboard port:
```sh
kubectl port-forward $(kubectl get pod -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070
```

### Advanced: Run on Docker for Mac as a core Nuclio developer, with an insecure registry
In this example we will install and run nuclio in the default namespace, for simplicity

Build the images locally (with your modified code) by running this on the repo root directory:
```sh
make build
```

Run a local Docker registry:
```sh
docker run -d -p 5000:5000 registry:2
```

Make sure your images are up to date and install the helm chart using the latest tag:
```sh
helm install nuclio \
    --set registry.pushPullUrl=localhost:5000 \
	--set controller.image.tag=latest-amd64 \
	--set dashboard.image.tag=latest-amd64 \
	--set dashboard.baseImagePullPolicy=Never \
	.
```

Forward the dashboard port:
```sh
kubectl port-forward $(kubectl get pod -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070
```

> Note: You can delete one (or both) of the deployments and run the service in the IDE. It will pick up the local kubeconfig file
