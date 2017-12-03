# Getting Started with nuclio on Google Kubernetes Engine (GKE)

Follow this step-by-step guide to set up a nuclio development environment that uses the [Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/) and related [Google Cloud Platform (GCP)](https://cloud.google.com/) tools.

#### In this document

- [Prerequisites](#prerequisites)
- [Set up a Kubernetes cluster and local environment](#set-up-a-kubernetes-cluster-and-a-local-environment)
- [Deploy a function with the nuclio playground](#deploy-a-function-with-the-nuclio-playground)
- [Deploy a function with the nuclio CLI (nuctl)](#deploy-a-function-with-the-nuclio-cli-nuctl)

## Prerequisites

Before deploying nuclio to GKE, ensure that the following prerequisites are met:

1.  You have a billable GKE project. For detailed information about GKE, see the [GKE documentation](https://cloud.google.com/kubernetes-engine/docs/).

    > **Note:** For simplicity, this guide uses the GKE project name `nuclio-gke`. Replace all reference to this name with the name of your GKE project.

2. The GCP CLI, [`gcloud`](https://cloud.google.com/sdk/gcloud/), is installed and configured to work with your GKE project.

3. The GCR Docker credentials helper, [`docker-credential-gcr`](https://github.com/GoogleCloudPlatform/docker-credential-gcr), is installed. You can use this `gcloud` command to install it:

    ```sh
    gcloud components install docker-credential-gcr
    ```

4. The [Google Container Registry (GCR)](cloud.google.com/container-registry/) API is [enabled](https://console.cloud.google.com/flows/enableapi?apiid=cloudbuild.googleapis.com) on your project.

## Set up a Kubernetes cluster and a local environment

1.  **Create a Kubernetes cluster**: use `gcloud` to spin-up a Kubernetes cluster; feel free to modify the parameters:

    ```sh
    gcloud container clusters create nuclio --machine-type n1-standard-2 --image-type COS --disk-size 100 --num-nodes 2
    ```

2.  **Get the credentials of the cluster** by running the following `gcloud` command. This command updates the [_kubeconfig_](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) file, which configures access to your cluster:

    ```sh
    gcloud container clusters get-credentials nuclio
    ```

    Run the following `kubectl` command to verify your configuration:

    ```sh
    kubectl get pods --all-namespaces
    ```

3.  **Punch a hole in the firewall**: at this time, `nuclio` creates a Kubernetes [ingress](/docs/concepts/k8s/function-ingress.md) for each function. GKE has a very low limit for ingress resources (five), because each ingress resource allocates a reverse-proxy IP. To work around this, until a [fan-out ingress](https://cloud.google.com/container-engine/docs/tutorials/http-balancer) option is implemented, you will access functions through their node ports (NodePort). To do this, use the following `gcloud` command to punch a hole in the firewall:

    ```sh
    gcloud compute firewall-rules create nuclio-nodeports --allow tcp:30000-32767
    ```

    It is recommended to add the `--source-ranges` option to this command, so as to limit who can access your cluster's node ports. For more information, see the [GCP firewall-rules documentation](https://cloud.google.com/vpc/docs/using-firewalls).

4.  **Deploy the nuclio controller**, which watches for new functions, by running the following `kubectl` command:

    ```sh
    kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/controller.yaml
    ```

## Deploy a function with the nuclio playground

The [nuclio playground](/README.md#playground) builds and pushes functions to a Docker registry. To use GCR, you'll need to set up a secret and mount it to the playground container, so that it can authenticate its Docker client against GCR. Start by getting your service ID:

```sh
gcloud iam service-accounts list
```

> **Note:** For simplicity, the service account in this guide is named `1234-compute@developer.gserviceaccount.com`. Replace all instances of this name with the name of your service account.

Create a service-to-service key, allowing GKE to access GCR. This guide uses the key `gcr.io`. You can replace this with any of the supported sub domains, such as `us.gcr.io` if you want to force the US region:

```sh
gcloud iam service-accounts keys create _json_key---gcr.io.json --iam-account 1234-compute@developer.gserviceaccount.com
```

Create the Kubernetes secret from the service-key file, and delete the file:

```sh
kubectl create secret generic nuclio-docker-keys --from-file=_json_key---gcr.io.json
rm _json_key---gcr.io.json
```

Create a `configmap` file that will be used by the playground to determine which repository should be used for pushing and pulling images; replace `nuclio-gke`, if applicable:

```sh
kubectl create configmap nuclio-registry --from-literal=registry_url=gcr.io/nuclio-gke
```

Now, you can deploy the playground and access it on port 32050 of a relevant node IP (`kubectl describe node | grep ExternalIP`):

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/gke/playground.yaml
```

## Deploy a function with the nuclio CLI (nuctl)

<a id="go-supported-version"></a>First, ensure that you have v1.8 or later of the Go (Golang) programming language (see https://golang.org/doc/install), and Docker (see https://docs.docker.com/engine/installation). Then, create a Go workspace (for example, in **~/nuclio**):

```sh
export GOPATH=~/nuclio && mkdir -p $GOPATH
```

Now, build [`nuctl`](/docs/reference/nuctl/nuctl.md), the nuclio command-line tool (CLI), and add `$GOPATH/bin` to the path for this session:

```sh
go get -u github.com/nuclio/nuclio/cmd/nuctl
PATH=$PATH:$GOPATH/bin
```

Configure your local Docker environment so that it's able to push images to GCR (to which `nuctl` will be instructed to push). This will update your Docker's **config.json** file:

```sh
docker-credential-gcr configure-docker
```

Deploy the `helloworld` Go sample function; you can add the `--verbose` flag if you want to peek under the hood:

```sh
nuctl deploy -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go helloworld --registry gcr.io/nuclio-gke
```

And finally, execute the function (forced via NodePort because ingress takes a couple of minutes to initialize):

```sh
nuctl invoke helloworld --via external-ip
```

## What's next?

See the following resources to make the best of your new nuclio environment:

1. [Configuring a function](/docs/concepts/configuring-a-function.md)
2. [Invoking functions by name with an ingress](/docs/concepts/k8s/function-ingress.md)
3. [More function examples](/hack/examples/README.md)

