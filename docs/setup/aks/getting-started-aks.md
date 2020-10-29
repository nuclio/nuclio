# Getting Started with Nuclio on Azure Container Service (AKS)

Microsoft's [Azure Container Service (AKS)](https://azure.microsoft.com/services/container-service/) manages your hosted Kubernetes environment, making it quick and easy to deploy and manage containerized applications without container orchestration expertise. It also eliminates the burden of ongoing operations and maintenance by provisioning, upgrading, and scaling resources on demand, without taking your applications offline. For more information, see the [AKS documentation](https://docs.microsoft.com/azure/aks/).

Follow this step-by-step guide to set up a Nuclio development environment that uses Azure Container Service (AKS).

## In this document

- [Prerequisites](#prerequisites)
- [Set up your AKS cluster](#set-up-your-aks-cluster)
- [Create a container registry using the Azure CLI](#create-a-container-registry-using-the-azure-cli)
- [Grant Kubernetes and Nuclio access to the ACR](#grant-kubernetes-and-nuclio-access-to-the-acr)
- [Install Nuclio](#install-nuclio)
- [Deploy a function with the Nuclio dashboard](#deploy-a-function-with-the-nuclio-dashboard)
- [What's next](#whats-next)

## Prerequisites

Before starting the set-up procedure, ensure that the following prerequisites are met:

- You have an Azure account. If you don't have an account, you can [create one for free](https://azure.microsoft.com/free/).
- The [Azure CLI](https://docs.microsoft.com/cli/azure/) (`az`) is installed on your installation machine.
    See the [Azure CLI installation guide](https://docs.microsoft.com/cli/azure/install-azure-cli).

## Set up your AKS cluster

1.  <a id="create-resource-group"></a>**Create a resource group** by running the following `az` command (see the [Azure CLI documentation](https://docs.microsoft.com/cli/azure/group#az_group_create)):

    ```sh
    az group create --name <resource-group-name> --location <location>
    ```

    The following example creates a resource group named "my-nuclio-k8s-rg" that is located in western Europe (location "westeurope"):
    ```sh
    az group create --name my-nuclio-k8s-rg --location westeurope
    ```

2.  <a id="create-k8s-cluster"></a>**Create a Kubernetes cluster** by running the following `az` command (see the [Azure CLI documentation](https://docs.microsoft.com/cli/azure/aks#az_aks_create)):

    ```sh
    az aks create --resource-group <resource-group-name> --name <cluster-name> --node-count <number>
    ```

    The following example creates a cluster named "myNuclioCluster" in the "my-nuclio-k8s-rg" resource group that was created in the example in the previous step:
    ```sh
    az aks create --resource-group my-nuclio-k8s-rg --name myNuclioCluster --node-count 2 --generate-ssh-keys
    ```

    After several minutes, the deployment completes and returns information about the AKS deployment, in JSON format.

3.  <a id="install-kubectl-cli"></a>**Install the kubectl CLI**.
    If the CLI is already installed, you can skip to the [next step](#connect-aks-cluster-to-kubectl).
    The [`kubectl`](https://kubernetes.io/docs/user-guide/kubectl-overview/) Kubernetes command-line application enables you to connect to the Kubernetes cluster from your client computer.
    To install `kubectl` locally, run the following `az` command (see the [Azure CLI documentation](https://docs.microsoft.com/cli/azure/aks#az_aks_install_cli)):

    ```sh
    az aks install-cli
    ```

4.  <a id="connect-aks-cluster-to-kubectl"></a>**Connect to the cluster with kubectl** by running the following `az` command, which configures the `kubectl` CLI to connect to your Kubernetes cluster (see the [Azure CLI documentation](https://docs.microsoft.com/cli/azure/aks#az_aks_get_credentials)):

    ```sh
    az aks get-credentials --resource-group=<resource-group-name> --name=<cluster-name>
    ```

    For example, the following command gets the credentials of a cluster named "myNuclioCluster" in the "my-nuclio-k8s-rg" resource group that was created in the examples in the previous steps:
    ```sh
    az aks get-credentials --resource-group=my-nuclio-k8s-rg --name=myNuclioCluster
    ```

5.  <a id="verify-connection-to-cluster"></a>**Verify the connection to your cluster** by running the following `kubectl` command (see the [Kubernetes documentation](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#get)):

    ```sh
    kubectl get nodes
    ```

    The output is expected to resemble the following example:
    ```sh
    NAME                             STATUS    AGE       VERSION
    k8s-myNuclioCluster-36346190-0   Ready     49m       v1.7.7
    ```

## Create a container registry using the Azure CLI

[Azure Container Registry (ACR)](https://azure.microsoft.com/services/container-registry/) is a managed Docker container registry service that's used for storing private container container images. For more information, see the [ACR documentation](https://docs.microsoft.com/azure/container-registry/).
Microsoft's [Create a container registry using the Azure CLI](https://docs.microsoft.com/azure/container-registry/container-registry-get-started-azure-cli) guide explains how to use the `az` CLI to create a container registry.

The Nuclio dashboard builds and pushes functions to a Docker registry. For the Nuclio ACR setup, ACR serves as the Docker registry. Create an ACR instance by using the `az acr create` command (see the [Azure CLI documentation](https://docs.microsoft.com/cli/azure/acr#az_acr_create)):
> **Note:** The name of the registry (`<registry-name>`) must be unique.
```sh
az acr create --resource-group <resource-group-name> --name <registry-name> --sku Basic
```

The following example creates a registry named "mynuclioacr" in the "my-nuclio-k8s-rg" resource group:
```sh
az acr create --resource-group my-nuclio-k8s-rg --sku Basic --name mynuclioacr
```

## Grant Kubernetes and Nuclio access to the ACR

To grant the AKS Kubernetes cluster and the Nuclio dashboard access to the Azure Container Registry (ACR), as part of the [Nuclio installation](#install-nuclio) you'll need to create a secret that stores the registry credentials. You can select between the following two methods for authenticating with the ACR:

- [Service principal](#service-principal)
- [Admin account](#admin-account)

> **Note:** The admin-account method has some security concerns, including no option to assign roles. Therefore, it's considered better practice to create a service principal.

### Service principal

You can assign a [service principal](https://docs.microsoft.com/azure/active-directory/develop/active-directory-application-objects) to your registry, and use it from your application or service to implement headless authentication.

You can use the following command to create a service principal:

```sh
az ad sp create-for-rbac --scopes /subscriptions/<subscription-id>/resourcegroups/<resource-group-name>/providers/Microsoft.ContainerRegistry/registries/<registry-name> --role Contributor --name <service-prinicpal-name>
```

For example, the following command creates a service principal for a container registry named "mynuclioacr" in the "my-nuclio-k8s-rg" resource group:
```sh
az ad sp create-for-rbac --role Contributor --scopes /subscriptions/$(az account show --query id -o tsv)/resourcegroups/my-nuclio-k8s-rg/providers/Microsoft.ContainerRegistry/registries/mynuclioacr --name mynuclioacr-sp
```

Make a note of the username (the service principal's `clientID`) and the password, as you'll need them when you install Nuclio.

### Admin account

Each container registry includes an admin user account, which is disabled by default. You can enable the admin user and manage its credentials in the [Azure portal](https://docs.microsoft.com/azure/container-registry/container-registry-get-started-portal#create-a-container-registry) or by using the Azure CLI.

## Install Nuclio

At this stage you should have a functioning Kubernetes cluster, a Docker registry, and a working Kubernetes CLI (`kubectl`), and you can proceed to install the Nuclio services on the cluster (i.e., deploy Nuclio).

**Create a Nuclio namespace** by running the following command:

> **Note:** All Nuclio resources go into the "nuclio" namespace, and role-based access control (RBAC) is configured accordingly.

```sh
kubectl create namespace nuclio
```

**Create [a secret](#grant-kubernetes-and-nuclio-access-to-the-acr)** for authenticating Kubernetes and Nuclio with the ACR:

```sh
read -s mypassword
<enter your password>

kubectl create secret docker-registry registry-credentials --namespace nuclio \
    --docker-username <username> \
    --docker-password $mypassword \
    --docker-server <registry-name>.azurecr.io \
    --docker-email ignored@nuclio.io

unset mypassword
```

**Create the RBAC roles** that are required for using Nuclio:
> **Note:** You are encouraged to look at the [**nuclio-rbac.yaml**](https://github.com/nuclio/nuclio/blob/master/hack/k8s/resources/nuclio-rbac.yaml) file that's used in the following command before applying it, so that you don't get into the habit of blindly running things on your cluster (akin to running scripts off the internet as root).

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio-rbac.yaml
```

**Deploy Nuclio to the cluster:** the following command deploys the Nuclio controller and dashboard and the [Træfik](https://docs.traefik.io/) ingress controller, among other resources:

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/aks/resources/nuclio.yaml
```

Use the command `kubectl get pods --namespace nuclio` to verify both the controller and dashboard are running.

**Forward the Nuclio dashboard port:** the Nuclio dashboard publishes a service at port 8070. To use the dashboard, you first need to forward this port to your local IP address:
```sh
kubectl port-forward -n nuclio $(kubectl get pods -n nuclio -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070
```

**Forward the Træfik port:** to use Træfik as an ingress, you'll need to forward its port as well:
```sh
kubectl port-forward -n kube-system $(kubectl get pod -n kube-system -l k8s-app=traefik-ingress-lb -o jsonpath='{.items[0].metadata.name}') 8080:80
```

<a id="deploy-a-function-with-the-nuclio-dashboard"></a>
## Deploy a function with the Nuclio dashboard

Browse to `http://localhost:8070` (after having forwarded this port as part of the Nuclio installation) to see the [Nuclio dashboard](/README.md#dashboard).
Select the "default" project and then select **New Function** from the action toolbar to display the **Create function** page (http://localhost:8070/projects/default/create-function).
Choose one of the predefined template functions, and select **Deploy**. 
The first build populates the local Docker cache with base images and other files, so it might take a while to complete, depending on your network.
When the function deployment completes, you can select **Test** to invoke the function with a body.

## What's next?

See the following resources to make the best of your new Nuclio environment:

- [Deploying Functions](/docs/tasks/deploying-functions.md)
- [Invoking Functions by Name with a Kubernetes Ingress](/docs/concepts/k8s/function-ingress.md)
- [More function examples](/hack/examples/README.md)
- [References](/docs/reference/)
- [Best Practices and Common Pitfalls](/docs/concepts/best-practices-and-common-pitfalls.md)

