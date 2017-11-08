# Getting Started With nuclio On Google Container Engine (GKE) and Google Container Registry (GCR)

Before deploying nuclio to GKE, please make sure that:
1. You've set up a billable project in GKE (in this guide the project name is `nuclio-gke`)
2. `gcloud` is installed and configured to work with that project
3. You've installed the docker credentials helper (`gcloud components install docker-credential-gcr`)
4. You enabled [Container Registry API](https://console.cloud.google.com/flows/enableapi?apiid=cloudbuild.googleapis.com) on the project

## Setting Up a Cluster and Local Environment

Spin up a cluster (feel free to modify the parameters):

```bash
gcloud container clusters create nuclio --machine-type n1-standard-2 --image-type COS --disk-size 100 --num-nodes 2
```

Get the credentials of the cluster (updates `kubeconfig`):

```bash
gcloud container clusters get-credentials nuclio
```

You can test out your environment by making sure `kubectl get pods` returns successfully.

At this time, `nuclio` creates an ingress per function and GKE has a very low limit on Ingress resources (5, since each Ingress resource allocates a reverse proxy IP). To work around this until a [fan out ingress](https://cloud.google.com/container-engine/docs/tutorials/http-balancer) option is implemented we will access functions through their node port. To do this, we need to punch a hole in the firewall:

```bash
gcloud compute firewall-rules create nuclio-nodeports --allow tcp:30000-32767
```

It is recommended to apply `--source-ranges` to the above so as to limit who can access your cluster's node ports.

Finally, we'll deploy the nuclio controller which watches for new functions:

```bash
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/controller.yaml
```

## Deploy a Function from Playground

The nuclio playground builds and pushes functions to a docker registry. To use GCR, we'll need to set up a secret and mount it to the playground container so that it can authenticated its docker client against GCR. Start by getting your service ID:

```bash
gcloud iam service-accounts list
```

For the sake of this guide, the service account will be called `1234-compute@developer.gserviceaccount.com`. Create a service to service key, allowing GKE to access GCR (in this guide we'll use `gcr.io` - you can replace this with any of the sub-domains, e.g. `us.gcr.io` if you want to force a specific region):

```bash
gcloud iam service-accounts keys create _json_key---gcr.io.json --iam-account 1234-compute@developer.gserviceaccount.com
```

Create the kubernetes secret from the key file and delete the file:

```bash
kubectl create secret generic nuclio-docker-keys --from-file=_json_key---gcr.io.json
rm _json_key---gcr.io.json
```

Create a configmap so that the playground can know which repository it should push and pull from (replace `nuclio-gke` if applicable):

```bash
kubectl create configmap nuclio-registry --from-literal=registry_url=gcr.io/nuclio-gke
```

Now we can deploy the playground and access it on some node IP (`kubectl describe node | grep ExternalIP`) port 32050:

```bash
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/gke/playground.yaml
```


## Deploy a Function from nuctl

First, make sure you have Golang 1.8+ (https://golang.org/doc/install) and Docker (https://docs.docker.com/engine/installation). Create a Go workspace (e.g. in `~/nuclio`):

```bash
export GOPATH=~/nuclio && mkdir -p $GOPATH
```

Now build nuctl, the nuclio command line tool and add `$GOPATH/bin` to path for this session:
```bash
go get -u github.com/nuclio/nuclio/cmd/nuctl
PATH=$PATH:$GOPATH/bin
```

Configure your local docker environment so that it's able to push images to GCR (that's where nuctl will be told to push to). This will update your docker's `config.json`:

```bash
docker-credential-gcr configure-docker
```

Deploy the Golang hello world example (you can add `--verbose` if you want to peek under the hood):
```bash
nuctl deploy -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go helloworld --registry gcr.io/nuclio-gke
```

And finally execute it (force via node port since ingress takes a couple of minutes to initialize):
```bash
nuctl invoke helloworld --via external-ip
```
