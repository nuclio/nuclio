# Nuclio

##  High-Performance Serverless Event and Data Processing Platform

[Nuclio](https://nuclio.io) is a new "serverless" project, derived from [Iguazio](https://iguazio.com)'s elastic data life-cycle management service for high-performance events and data processing

## Introduction

This chart bootstraps a Nuclio deployment (controller and playground) and service on a Kubernetes cluster using the Helm Package manager. Before you get started you will need:

- A Kubernetes 1.7+ cluster with tiller installed
- helm 
- kubectl

### Adding Nuclio to helm
Until the Nuclio helm chart is accepted into the upstream repository, we must start by adding the Nuclio repository to helm:

```sh
helm repo add nuclio https://nuclio.github.io/nuclio/charts
```

## Installing Nuclio
The Nuclio helm chart allows a range of options that allow installation across different Kubernetes providers. Before you go ahead and run the appropriate installation command suitable for your environment, you must first create a registry secret to hold the credentials of your image registry. While this helm chart supports doing this for you (see `secretName` in `values.yaml` for instructions), we recommend you do this yourself.

> Note: You can skip this if you're using Minikube with an insecure registry

Start by creating a namespace:
``` sh
kubectl create namespace nuclio
```

Create the secret:
``` sh
read -s mypassword
<enter your password>

kubectl create secret docker-registry registry-credentials --namespace nuclio \
    --docker-username <username> \
    --docker-password $mypassword \
    --docker-server <registry name> \
    --docker-email ignored@nuclio.io

unset mypassword
```

### Install on Minikube, vanilla Kubernetes, AKS
There are no special flags required when installing in AKS or vanilla Kubernetes:

``` sh
helm install nuclio/nuclio
```

### Install on GKE (or when using GCR)
If you're using GCR as your image registry, there is a small quirk where the login URL is different from the push/pull URL. By default, Nuclio will take the push/pull URL from the secret, but in this case we need to let Nuclio know what the push/pull URL is:

``` sh
helm install \
	--set registry.pushPullUrl gcr.io/<your project name> \
	nuclio/nuclio
```

### Install on Minikube using a local, insecure registry
By clearing `registry.secretName`, Nuclio will not try to load Docker secrets.

``` sh
helm install \
	--set registry.secretName= \
	nuclio/nuclio
```

### Advanced: Run on Docker for Mac as a core Nuclio developer
Make sure your images are up to date and install the helm chart using the latest tag:
```sh
helm install \
	--set registry.secretName= \
	--set controller.image.tag=latest-amd64 \
	--set dashboard.image.tag=latest-amd64 \
	--set controller.baseImagePullPolicy=Never \
	--set dashboard.baseImagePullPolicy=Never \
	.
```

You will need to run a local Docker registry. Run the following command on the host if you're working with Docker for Mac or on the Minikube VM:
```sh
docker run -d -p 5000:5000 registry:2
```

Forward the dashboard port
```sh
kubectl port-forward $(kubectl get pod -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070
```

> Note: You can delete one (or both) of the deployments and run the service in the IDE. It will pick up the local kubeconfig file

## Configuration
TODO
