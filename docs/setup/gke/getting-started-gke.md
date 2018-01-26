# Getting Started with nuclio on Google Kubernetes Engine (GKE)

Follow this step-by-step guide to set up a nuclio development environment that uses the [Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/) and related [Google Cloud Platform (GCP)](https://cloud.google.com/) tools.

#### In this document

- [Prerequisites](#prerequisites)
- [Set up a Kubernetes cluster and local environment](#set-up-a-kubernetes-cluster-and-a-local-environment)
- [Install Nuclio](#install-nuclio)
- [Deploy a function with the nuclio playground](#deploy-a-function-with-the-nuclio-playground)
- [Deploy a function with the nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli-nuctl)

## Prerequisites

Before deploying nuclio to GKE, ensure that the following prerequisites are met:

You have a billable GKE project. For detailed information about GKE, see the [GKE documentation](https://cloud.google.com/kubernetes-engine/docs/).
> Note: For simplicity, this guide uses the GKE project name `nuclio-gke`. Replace all reference to this name with the name of your GKE project.

The GCP CLI, [`gcloud`](https://cloud.google.com/sdk/gcloud/), is installed and configured to work with your GKE project.

The GCR Docker credentials helper, [`docker-credential-gcr`](https://github.com/GoogleCloudPlatform/docker-credential-gcr), is installed. You can use this `gcloud` command to install it:

```sh
gcloud components install docker-credential-gcr
```

The [Google Container Registry (GCR)](cloud.google.com/container-registry/) API is [enabled](https://console.cloud.google.com/flows/enableapi?apiid=cloudbuild.googleapis.com) on your project.

## Set up a Kubernetes cluster and a local environment

Create a Kubernetes cluster: use `gcloud` to spin-up a Kubernetes cluster; feel free to modify the parameters:

```sh
gcloud container clusters create nuclio --machine-type n1-standard-2 --image-type COS --disk-size 100 --num-nodes 2
```

Get the credentials of the cluster by running the following `gcloud` command. This command updates the [_kubeconfig_](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) file, which configures access to your cluster:

```sh
gcloud container clusters get-credentials nuclio
```

Run the following `kubectl` command to verify your configuration:

```sh
kubectl get pods --all-namespaces
```

The [nuclio playground](/README.md#playground) builds and pushes functions to a Docker registry. To use GCR, you'll need to set up a secret and mount it to the playground container, so that it can authenticate its Docker client against GCR. Start by getting your service ID:

```sh
gcloud iam service-accounts list
```

> Note: For simplicity, the service account in this guide is named `1234-compute@developer.gserviceaccount.com`. Replace all instances of this name with the name of your service account.

Create a service-to-service key, allowing GKE to access GCR. This guide uses the key `gcr.io`. You can replace this with any of the supported sub domains, such as `us.gcr.io` if you want to force the US region:

```sh
gcloud iam service-accounts keys create credentials.json --iam-account 1234-compute@developer.gserviceaccount.com
```

## Install nuclio
By now you should have a functioning Kubernetes cluster, credentials to a private Docker registry and a working Kubernetes CLI (kubectl). Go ahead and install the nuclio services on the cluster.
> Note: All nuclio resources go into the "nuclio" namespace and RBAC is configured accordingly

Start by creating a namespace for nuclio:
```sh
kubectl create namespace nuclio
```

Create a Kubernetes docker registry secret from the service-key file we create prior and delete the file:
 
```sh
kubectl create secret docker-registry registry-credentials --namespace nuclio \
    --docker-username _json_key \
    --docker-password "$(cat credentials.json)" \
    --docker-server gcr.io \
    --docker-email ignored@nuclio.io
    
rm credentials.json
```

Create a `configmap` file that will be used by the playground to determine which repository should be used for pushing and pulling images:

```sh
kubectl create configmap --namespace nuclio nuclio-registry --from-literal=registry_url=gcr.io/$(gcloud config list --format 'value(core.project)')
```

Create the RBAC roles necessary for nuclio:
> Note: You are encouraged to peek at the file before applying it, so that you don't get into the habit of blindly running things on your cluster (akin to running scripts off the internet as root).

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio-rbac.yaml
```

Now deploy nuclio (deploys the controller and the playground, among other resources):
```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/gke/resources/nuclio.yaml
```

Use `kubectl get po --namespace nuclio` to verify both the controller and playground are running and port forward the playground port:  
 
```sh
kubectl port-forward -n nuclio $(kubectl get pod -n nuclio -l nuclio.io/app=playground -o jsonpath='{.items[0].metadata.name}') 8070:8070
```

Browse to `http://localhost:8070`. You should be greeted by the [nuclio playground](/README.md#playground). Choose one of the built-in examples, and click **Deploy**. The first build will populate the local Docker cache with base images and other files, so it might take a while, depending on the network. When the function deployment is completed, you can click **Invoke** to invoke the function with a body.

## Deploy a function with the nuclio CLI (nuctl)

Start by downloading the latest [nuctl](https://github.com/nuclio/nuclio/releases) for your platform and then deploy the `helloworld` Go sample function. You can add the `--verbose` flag if you want to peek under the hood:
> Note: If you are using Docker hub, the URL here includes your username: `registry.hub.docker.com/<username>`

```sh
nuctl deploy helloworld -n nuclio -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry <URL>
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
