# nuclio

##  High-Performance Serverless Event and Data Processing Platform

[nuclio](https://nuclio.io) is a new "serverless" project, derived from [Iguazio](https://iguazio.com)'s elastic data life-cycle management service for high-performance events and data processing

## Introduction

This chart bootstraps a nuclio deployment (controller and playground) and service on a Kubernetes cluster using the Helm Package manager. Before you get started you will need:

- A Kubernetes 1.7+ cluster with tiller installed
- helm 
- kubectl

### Adding nuclio to helm
Until the nuclio helm chart is accepted into the upstream repository, we must start by adding the nuclio repository to helm:

```sh
helm repo add nuclio https://nuclio.github.io/nuclio/charts
```

## Installing nuclio
The nuclio helm chart allows a range of options that allow installation across different Kubernetes providers. Before you go ahead and run the appropriate installation command suitable for your environment, you must first create a registry secret to hold the credentials of your image registry. While this helm chart supports doing this for you (see `secretName` in `values.yaml` for instructions), we recommend you do this yourself.

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
If you're using GCR as your image registry, there is a small quirk where the login URL is different from the push/pull URL. By default, nuclio will take the push/pull URL from the secret, but in this case we need to let nuclio know what the push/pull URL is:

``` sh
helm install \
	--set registry.pushPullUrl gcr.io/<your project name> \
	nuclio/nuclio
```

### Install on Minikube using a local, insecure registry
By clearing `registry.secretName`, nuclio will not try to load Docker secrets.

``` sh
helm install \
	--set registry.secretName= \
	nuclio/nuclio
```

### Advanced: Install on Minikube as a core nuclio developer
If you plan to develop the nuclio core (see /docs/devel/minikube/developing-on-minikube.md), run the command thusly:

```
helm install \
	--set registry.secretName= \
	--set controller.image.tag=latest-amd64 \
	--set dashboard.image.tag=latest-amd64 \
	--set controller.baseImagePullPolicy=Never \
	--set dashboard.baseImagePullPolicy=Never \
	.
```

## Configuration
TODO
