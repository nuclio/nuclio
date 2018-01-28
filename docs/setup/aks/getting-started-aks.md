# Getting Started with nuclio on Azure Container Service (AKS)

Azure Container Service (AKS) manages your hosted Kubernetes environment, making it quick and easy to deploy and manage containerized applications without container orchestration expertise. It also eliminates the burden of ongoing operations and maintenance by provisioning, upgrading, and scaling resources on demand, without taking your applications offline. [Read more about Azure Container Service (AKS)](https://docs.microsoft.com/en-us/azure/aks/).

Follow this step-by-step guide to set up a nuclio development environment that uses Azure Container Service (AKS).

## In this document

- [Prerequisites](#prerequisites)
- [Set up your AKS cluster](#set-up-your-aks-cluster)
- [Create a container registry using the Azure CLI](#create-a-container-registry-using-the-azure-cli)
- [Granting Kubernetes access to ACR](#granting-kubernetes-access-to-acr)
- [Install Nuclio](#install-nuclio)
- [Deploy a function with the nuclio playground](#deploy-a-function-with-the-nuclio-playground)

## Prerequisites

1. You will need an Azure account. If you don't have an account, you can [create one for free](https://azure.microsoft.com/en-us/free/).

2. Install [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest).

## Set up your AKS cluster

**Create a reource group**: 

```sh
az group create --name <resource-group-name> --location <location>
```
For example:
```sh
az group create --name my-nuclio-k8s-rg --location westeurope
```
**Create Kuberenetes Cluster**: The following snippet creates a cluster named `myNuclioCluster` in a Resource Group named `my-nuclio-k8s-rg`. This Resource Group was created in the previous step. [Read more about creating cluster options here](https://docs.microsoft.com/en-us/cli/azure/aks?view=azure-cli-latest#az_aks_create).

```sh
az aks create --resource-group <resource-group-name> --name <cluster-name> --node-count <number>
```
For example:
```sh
az aks create --resource-group my-nuclio-k8s-rg --name myNuclioCluster --node-count 2 --generate-ssh-keys
```
After several minutes, the deployment completes, and returns json formatted information about the AKS deployment.

**Install the kubectl CLI**: Optional if you don't have it already installed. To connect to the Kubernetes cluster from your client computer, use [kubectl](https://kubernetes.io/docs/user-guide/kubectl/), the Kubernetes command-line client. To install it locally, run the following command:
```sh
az aks install-cli
```

**Connect with kubectl**: To configure kubectl to connect to your Kubernetes cluster, run the following command:
```sh
az aks get-credentials --resource-group=<resource-group-name> --name=<cluster-name>
```
For example:
```sh
az aks get-credentials --resource-group=my-nuclio-k8s-rg --name=myNuclioCluster
```

**Verify connection to your cluster**: Run the kubectl get nodes command:

```sh
kubectl get nodes
```
The expected output should be similar to:
```sh
NAME                             STATUS    AGE       VERSION
k8s-myNuclioCluster-36346190-0   Ready     49m       v1.7.7
```

## Create a container registry using the Azure CLI
[Azure Container Registry (ACR)](https://docs.microsoft.com/en-us/azure/container-registry/container-registry-get-started-azure-cli) is a managed Docker container registry service used for storing private Docker container images.

The nuclio playground builds and pushes functions to a Docker registry. In our case we use ACR as our Docker registry. Create an ACR instance using the [az acr create](https://docs.microsoft.com/en-us/cli/azure/acr#az_acr_create) command. The name of the registry **must be unique**. In the following example `mynuclioacr` is used. Update this to a unique value.
```sh
az acr create --resource-group <resource-group-name> --name <registry-name> --sku Basic
```
For example:
```sh
az acr create --resource-group my-nuclio-k8s-rg --sku Basic --name mynuclioacr 
```

## Granting Kubernetes and nuclio access to ACR
To use ACR, you'll need to set up a secret so that AKS and the playground can access it. 
There are 2 ways to authenticate with an Azure container registry for our case:
- Service principal. You can assign a [service principal](https://docs.microsoft.com/en-us/azure/active-directory/develop/active-directory-application-objects) to your registry, and your application or service can use it for headless authentication
- Admin account. Each container registry includes an admin user account, which is disabled by default. You can enable the admin user and manage its credentials in the [Azure portal](https://docs.microsoft.com/en-us/azure/container-registry/container-registry-get-started-portal#create-a-container-registry), or by using the Azure CLI
  
Using an admin account would forfeit the ability to assign roles among other security concerns, so it is considered better practice to create a service principal:  

```sh
az ad sp create-for-rbac --scopes /subscriptions/<subscription-id>/resourcegroups/<resource-group-name>/providers/Microsoft.ContainerRegistry/registries/<registry-name> --role Contributor --name <service-prinicpal-name>
```
For example:
```sh
az ad sp create-for-rbac --role Contributor --scopes /subscriptions/$(az account show --query id -o tsv)/resourcegroups/my-nuclio-k8s-rg/providers/Microsoft.ContainerRegistry/registries/mynuclioacr --name mynuclioacr-sp
```

Make note of the username (the service principal's `clientID`) and the password, as we'll need them when we install nuclio.

## Install nuclio
By now you should have a functioning Kubernetes cluster, a Docker registry, and a working Kubernetes CLI (kubectl). Now, you can go ahead and install the nuclio services on the cluster.
> Note: All nuclio resources go into the "nuclio" namespace and RBAC is configured accordingly

Start by creating a namespace for nuclio:
```sh
kubectl create namespace nuclio
```

Create the secret to be used by Kubernetes and nuclio for access to ACR:
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

Create the RBAC roles necessary for nuclio:
> Note: You are encouraged to peek at the file before applying it, so that you don't get into the habit of blindly running things on your cluster (akin to running scripts off the internet as root).

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/nuclio-rbac.yaml
```

Now deploy nuclio (deploys the controller, the playground and the traefik ingress controller, among other resources):
```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/aks/resources/nuclio.yaml
```

Use `kubectl get po --namespace nuclio` to verify both the controller and playground are running and port forward the playground port:  
 
```sh
kubectl port-forward -n nuclio $(kubectl get pod -n nuclio -l nuclio.io/app=playground -o jsonpath='{.items[0].metadata.name}') 8070:8070
```

To use Traefik as an ingress, we'll need to forward its port as well:

```sh
kubectl port-forward -n kube-system $(kubectl get pod -n kube-system -l k8s-app=traefik-ingress-lb -o jsonpath='{.items[0].metadata.name}') 8080:80
```

## Deploy a function with the nuclio playground

Browse to `http://localhost:8070`. You should be greeted by the [nuclio playground](/README.md#playground). Choose one of the built-in examples, and click **Deploy**. The first build will populate the local Docker cache with base images and other files, so it might take a while, depending on the network. When the function deployment is completed, you can click **Invoke** to invoke the function with a body.


## What's next?

See the following resources to make the best of your new nuclio environment:

1. [Configuring a function](/docs/concepts/configuring-a-function.md)
2. [Invoking functions by name with an ingress](/docs/concepts/k8s/function-ingress.md)
3. [More function examples](/hack/examples/README.md)

