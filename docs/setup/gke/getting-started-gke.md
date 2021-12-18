# Getting Started with Nuclio on Google Kubernetes Engine (GKE)

Follow this step-by-step guide to set up a Nuclio development environment that uses the [Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/) and related [Google Cloud Platform (GCP)](https://cloud.google.com/) tools.

#### In this document

- [Prerequisites](#prerequisites)
- [Set up a Kubernetes cluster and a local environment](#set-up-a-kubernetes-cluster-and-a-local-environment)
- [Install Nuclio](#install-nuclio)
- [What's next](#whats-next)

## Prerequisites

Before starting the set-up procedure, ensure that the following prerequisites are met:

- You have a billable GKE project. For detailed information about GKE, see the [GKE documentation](https://cloud.google.com/kubernetes-engine/docs/).

    > **Note:** For simplicity, this guide uses the GKE project name `nuclio-gke`. Replace all reference to this name with the name of your GKE project.

- The [GCP CLI](https://cloud.google.com/sdk/gcloud/) (`gcloud`) is installed and configured to work with your GKE project.

- The [GCR Docker credentials helper](https://github.com/GoogleCloudPlatform/docker-credential-gcr), (`docker-credential-gcr`) is installed. You can use this `gcloud` command to install it:

    ```sh
    gcloud components install docker-credential-gcr
    ```

- The [Google Container Registry (GCR)](https://cloud.google.com/container-registry/) API is [enabled](https://console.cloud.google.com/flows/enableapi?apiid=cloudbuild.googleapis.com) on your project.

- The [Nuclio CLI](/docs/reference/nuctl/nuctl.md) (`nuctl`) is installed &mdash; if you wish to [use the CLI to deploy Nuclio functions](#deploy-a-function-with-the-nuclio-cli).
    To install the CLI, simply [download](https://github.com/nuclio/nuclio/releases) the appropriate CLI version to your installation machine.

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
> **Note:** The first command, which sets `GKE_USER`, is just a method for retrieving your registered email address. This command requires `jq`. If you know your GCP registration email address, you can enter it manually; note that the email address is case sensitive.

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

**Create a secret for GCR authentication:** because Nuclio functions are images that need to be pushed and pulled to/from the registry, you need to create a secret that stores your registry credentials, and mount the secret to the Nuclio dashboard container so that it can be used to authenticate the Docker client with the GCR. Start by getting your service ID.

> **Note:** You can use any private Docker registry:
>
> - To use the Azure Container Registry (ACR), see [Getting Started with Nuclio on Azure Container Service (AKS)](/docs/setup/aks/getting-started-aks.md).
> - To use the Docker Hub, see [Getting Started with Nuclio on Kubernetes](/docs/setup/k8s/getting-started-k8s.md).
> - For other registries, create a Docker-registry secret named `registry-credentials` for storing your registry credentials. If the registry URL differs from the URL in the credentials, create a ConfigMap file named **nuclio-registry** that contains the URL, as demonstrated in the [Nuclio installation](#install-nuclio) instructions later in this guide.

**Create a service-to-service key that allows GKE to access the GCR:** this guide uses the key `gcr.io`. You can replace this with any of the supported sub domains, such as `us.gcr.io` if you want to force the US region:

```sh
gcloud iam service-accounts keys create credentials.json --iam-account $(gcloud iam service-accounts list --format "value(email)")
```

## Install Nuclio

At this stage you should have a functioning Kubernetes cluster, credentials to a private Docker registry, and a working Kubernetes CLI (`kubectl`), and you can proceed to install the Nuclio services on the cluster (i.e., deploy Nuclio). For more information about `kubectl`, see the [Kubernetes documentation](https://kubernetes.io/docs/user-guide/kubectl-overview/).

**Create a Kubernetes Docker-registry secret** from service-key file that you created as part of the [Kubernetes cluster setup](#set-up-a-kubernetes-cluster-and-a-local-environment)

And then follow the instructions of [How to run nuclio in Production](/docs/setup/k8s/running-in-production-k8s.md#the-preferred-deployment-method)

> NOTE: use the below  docker registry secret creation command:

```sh
kubectl create secret docker-registry registry-credentials \
    --namespace nuclio \
    --docker-username _json_key \
    --docker-password "$(cat credentials.json)" \
    --docker-server gcr.io \
    --docker-email ignored@nuclio.io

rm credentials.json
```

Use the command `kubectl --namespace nuclio get pods` to verify both the controller and dashboard are running.

## What's next?

See the following resources to make the best of your new Nuclio environment:

- [Deploying Functions](/docs/tasks/deploying-functions.md)
- [Invoking Functions by Name with a Kubernetes Ingress](/docs/concepts/k8s/function-ingress.md)
- [More function examples](/hack/examples/README.md)
- [References](/docs/reference)
- [Best Practices and Common Pitfalls](/docs/concepts/best-practices-and-common-pitfalls.md)
