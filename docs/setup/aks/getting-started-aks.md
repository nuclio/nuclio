# Getting Started with nuclio on Azure Container Service (AKS)

Azure Container Service (AKS) manages your hosted Kubernetes environment, making it quick and easy to deploy and manage containerized applications without container orchestration expertise. It also eliminates the burden of ongoing operations and maintenance by provisioning, upgrading, and scaling resources on demand, without taking your applications offline. [Read more about Azure Container Service (AKS)](https://docs.microsoft.com/en-us/azure/aks/).

Follow this step-by-step guide to set up a nuclio development environment that uses Azure Container Service (AKS).

## In this document

- [Prerequisites](#prerequisites)
- [Set up your AKS cluster](#)
- [Create a container registry using the Azure CLI](#)
- [Granting Kubernetes access to ACR](#)
- [Install Nuclio](#)
- [Deploy a function with the nuclio playground](#)

## Prerequisites

1. You will need an Azure account. If you don't have an account, you can [create one for free](https://azure.microsoft.com/en-us/free/).

2. Install [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest).



## Set up your AKS cluster

1.  **Create a reource group**: 

    ```sh
    az group create --name my-nuclio-k8s-rg --location westeurope
    ```
2.  **Create Kuberenetes Cluster**: The following snippet creates a cluster named `myNuclioCluster` in a Resource Group named `my-nuclio-k8s-rg`. This Resource Group was created in the previous step.


    ```sh
    az aks create --resource-group my-nuclio-k8s --name myNuclioCluster --node-count 2 --generate-ssh-keys
    ```
    After several minutes, the deployment completes, and returns json formatted information about the AKS deployment.

3. **Install the kubectl CLI**: To connect to the Kubernetes cluster from your client computer, use [kubectl](https://kubernetes.io/docs/user-guide/kubectl/), the Kubernetes command-line client. To install it locally, run the following command:
```sh
az aks install-cli
```

4. **Connect with kubectl**: To configure kubectl to connect to your Kubernetes cluster, run the following command:
```sh
az aks get-credentials --resource-group=my-nuclio-k8s-rg --name=myNuclioCluster
```

5. **Verify connection to your cluster**: Run the kubectl get nodes command:
following command:
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

The nuclio playground builds and pushes functions to a Docker registry. In our case we use ACR as out Docker registry.

Create an ACR instance using the [az acr create](https://docs.microsoft.com/en-us/cli/azure/acr#az_acr_create) command.

The name of the registry **must be unique**. In the following example `myNuclioAcr` is used. Update this to a unique value.
```sh
az acr create --resource-group my-nuclio-k8s-rg --name myNuclioAcr --sku Basic
```

## Granting Kubernetes access to ACR
1. To use AKS, you'll need to set up a secret and mount it to the playground container, so that it can authenticate its Docker client against ACR. Start by creating a local directory on your machine, and inside create a new file for storing the credentials:
```
File name: <acr-user-name---<acr-server>.json
File content: <acr-password>
```

2. The expected format of the secret is a text file containing the password of the user specified by the file name. For example, if the secret name is `966e5820-5443-48c5-be62-b4d62798ab68---mynuclioacr.azurecr.io.json` and the contents are `aaabbbcccddd`, the playground will run "docker login" where:
```
- user: 966e5820-5443-48c5-be62-b4d62798ab68
- password: aaabbbcccddd
- server: mynuclioacr.azurecr.io (lowercase)
```

3. Use the CLI to navigate to the directory where you stored the secrets file. Create the Kubernetes secret from the service-key file, and delete the file:
```sh
kubectl create secret generic nuclio-docker-keys --from-file=966e5820-5443-48c5-be62-b4d62798ab68---mynuclioacr.azurecr.io.json

rm 966e5820-5443-48c5-be62-b4d62798ab68---mynuclioacr.azurecr.io.json
```

4. Create a `configmap` file that will be used by the playground to determine which repository should be used for pushing and pulling images:
```sh
kubectl create configmap nuclio-registry --from-literal=registry_url=mynuclioacr.azurecr.io
```
> **Note**: While the secret is used to trigger a docker login to a docker registry, the config map is used to pass a variable indicating which repository to push to. 

## Install Nuclio
After following your selected Kubernetes installation instructions, you should have a functioning Kubernetes cluster, a Docker registry, and a working local Kubernetes CLI (kubectl). Now, you can go ahead and install the nuclio services on the cluster.

1. Deploy the `nuclio controller`, which watches for new functions, by running the following kubectl command:
```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/controller.yaml
```

2. Now, you can deploy the `nuclio playground` and access it on port 32050 of a relevant node IP:
```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/development/hack/aks/resources/playground.yaml
```

3. Add `Traefik Load Balancer` to act as an Ingress Controller for your Nuclio Functions:
```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/development/hack/aks/resources/traefik-lb.yaml
```

Finaly, let’s wait until the services will be up and running.
Use the command `kubectl get pods` to verify that both the controller and playground have a status of `Running`. For more information about `kubectl`, see the [Kubernetes documentation](https://kubernetes.io/docs/user-guide/kubectl-overview/).

## Deploy a function with the nuclio playground

Browse to `http://$(kubectl describe node | grep ExternalIP):32050`.
You should be greeted by the [nuclio playground](/README.md#playground). Choose one of the built-in examples, and click **Deploy**. The first build will populate the local Docker cache with base images and other files, so it might take a while, depending on the network. When the function deployment is completed, you can click **Invoke** to invoke the function with a body.


## What's next?

See the following resources to make the best of your new nuclio environment:

1. [Configuring a function](/docs/concepts/configuring-a-function.md)
2. [Invoking functions by name with an ingress](/docs/concepts/k8s/function-ingress.md)
3. [More function examples](/hack/examples/README.md)

