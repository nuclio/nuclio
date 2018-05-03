# Getting Started with nuclio on Google Kubernetes Engine (GKE)

Follow this step-by-step guide to set up a nuclio development environment that uses the [Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/) and related [Google Cloud Platform (GCP)](https://cloud.google.com/) tools.

#### In this document

- [Prerequisites](#prerequisites)
- [Set up a Kubernetes cluster and a local environment](#set-up-a-kubernetes-cluster-and-a-local-environment)
- [Install nuclio](#install-nuclio)
- [Deploy a function with the nuclio dashboard](#deploy-a-function-with-the-nuclio-dashboard)
- [Deploy a function with the nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli-nuctl)
- [What's next](#whats-next)

## Prerequisites

Before deploying nuclio to GKE, ensure that the following prerequisites are met:

- You have a billable GKE project. For detailed information about GKE, see the [GKE documentation](https://cloud.google.com/kubernetes-engine/docs/).

    > Note: For simplicity, this guide uses the GKE project name `nuclio-gke`. Replace all reference to this name with the name of your GKE project.

- The GCP CLI, [`gcloud`](https://cloud.google.com/sdk/gcloud/), is installed and configured to work with your GKE project.

- The GCR Docker credentials helper, [`docker-credential-gcr`](https://github.com/GoogleCloudPlatform/docker-credential-gcr), is installed. You can use this `gcloud` command to install it:

    ```sh
    gcloud components install docker-credential-gcr
    ```

- The [Google Container Registry (GCR)](cloud.google.com/container-registry/) API is [enabled](https://console.cloud.google.com/flows/enableapi?apiid=cloudbuild.googleapis.com) on your project.

## Set up a Kubernetes cluster and a local environment

**Create a Kubernetes cluster** by running the following `gcloud` command. Feel free to modify the options and parameters:

```sh
gcloud container clusters create nuclio --machine-type n1-standard-2 --image-type COS --disk-size 100 --num-nodes 2 --no-enable-legacy-authorization
```

**Get the credentials of the cluster** by running the following `gcloud` command. This command updates the [**kubeconfig**](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) file, which configures access to your cluster:

```sh
gcloud container clusters get-credentials nuclio
```

**Assign cluster-admin permissions to your user** to allow creation of role-based access control (RBAC) roles, in accordance with the [GKE documentation](https://cloud.google.com/kubernetes-engine/docs/how-to/role-based-access-control):
> Note: The first command, which sets `GKE_USER`, is just a method for retrieving your registered email address. This command requires `jq`. If you know your GCP registration email address, you can enter it manually; note that the email address is case sensitive.

```sh
GKE_USER=$(gcloud projects get-iam-policy "$(gcloud config list --format 'value(core.project)')" --format json \
           | jq -r '.bindings[] | select(.role == "roles/owner") | .members[]' \
           | awk -F':' '{print $2}')

kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user $GKE_USER
```

**Verify your configuration** by running the following `kubectl` command (see the [Kubernetes documentation](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#get)):

```sh
kubectl get pods --all-namespaces
```

**Create a secret for GCR authentication:** because nuclio functions are images that need to be pushed and pulled to/from the registry, you need to create a secret that stores your registry credentials, and mount the secret to the nuclio dashboard container so that it can be used to authenticate the Docker client with the GCR. Start by getting your service ID.

> Note: You can use any private Docker registry:
>
> - To use the Azure Container Registry (ACR), see [Getting Started with nuclio on Azure Container Service (AKS)](/docs/setup/aks/getting-started-aks.md).
> - To use the Docker Hub, see [Getting Started with nuclio on Kubernetes](/docs/setup/k8s/getting-started-k8s.md).
> - For other registries, create a Docker-registry secret named `registry-credentials` for storing your registry credentials. If the registry URL differs from the URL in the credentials, create a ConfigMap file named **nuclio-registry** that contains the URL, as demonstrated in the [nuclio installation](#install-nuclio) instructions later in this guide.

**Create a service-to-service key that allows GKE to access the GCR:** this guide uses the key `gcr.io`. You can replace this with any of the supported sub domains, such as `us.gcr.io` if you want to force the US region:

```sh
gcloud iam service-accounts keys create credentials.json --iam-account $(gcloud iam service-accounts list --format "value(email)")
```

## Install nuclio

At this stage you should have a functioning Kubernetes cluster, credentials to a private Docker registry, and a working Kubernetes CLI (`kubectl`), and you can proceed to install the nuclio services on the cluster (i.e., deploy nuclio). For more information about `kubectl`, see the [Kubernetes documentation](https://kubernetes.io/docs/user-guide/kubectl-overview/).

**Create a nuclio namespace** by running the following command:

> Note: All nuclio resources go into the "nuclio" namespace, and role-based access control (RBAC) is configured accordingly.

```sh
kubectl create namespace nuclio
```

**Create a Kubernetes Docker-registry secret** from service-key file that you created as part of the [Kubernetes cluster setup](#set-up-a-kubernetes-cluster-and-a-local-environment), and delete this file:

```sh
kubectl create secret docker-registry registry-credentials --namespace nuclio \
    --docker-username _json_key \
    --docker-password "$(cat credentials.json)" \
    --docker-server gcr.io \
    --docker-email ignored@nuclio.io

rm credentials.json
```

**Create a registry configuration file:** create a **nuclio-registry** ConfigMap file that will be used by the nuclio dashboard to determine which repository should be used for pushing and pulling images:

```sh
kubectl create configmap --namespace nuclio nuclio-registry --from-literal=registry_url=gcr.io/$(gcloud config list --format 'value(core.project)')
```

**Create the RBAC roles** that are required for using nuclio:
> Note: You are encouraged to look at the [**nuclio-rbac.yaml**](https://github.com/nuclio/nuclio/blob/master/hack/k8s/resources/nuclio-rbac.yaml) file that's used in the following command before applying it, so that you don't get into the habit of blindly running things on your cluster (akin to running scripts off the internet as root).

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio-rbac.yaml
```

**Deploy nuclio to the cluster:** the cluster. The following command deploys the nuclio controller and dashboard, among other resources:

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/gke/resources/nuclio.yaml
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

