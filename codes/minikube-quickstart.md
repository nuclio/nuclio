
# Quickstart Guide on Developing Locally with Nuclio and Minikube

> **Disclaimer:** This guide is written for and was tested using MacOS 14.0. 
> There might (and probably will) be differences when using different operating systems.

## Requirements

| What     | Why                                               |
|----------|---------------------------------------------------|
| Git      | To download our version of the Nuclio source code |
| Make     | To build Nuclio                                   |
| Minikube | To run a local Kubernetes cluster                 |
| Kubectl  | To control/access the cluster                     |
| Helm     | To run Nuclio on the local cluster                |
  
Optionally, you can also use [nuctl](https://nuclio.io/docs/latest/reference/nuctl/nuctl/) to control Nuclio using your terminal instead of the dashboard.

----

## Setup

### 1. Getting the code

The first step in setting up Nuclio to run in a local Kubernetes cluster is to get a copy of the source code.
To do that, clone our Nuclio fork.
We mostly use the `1.11.x` branch (not `development`), as it is the latest stable release, but you might want to switch to a more specific feature branch.

```shell
git clone git@github.com:valentin-carl/nuclio.git
cd nuclio
git switch 1.11.x 
```

### 2. Setting up Minikube

Minikube is used to run a local Kubernetes cluster. 
If you already have a Minikube installation, you might want to use the two next commands to completely reset it.
If you already have a Minikube installation with data you want to keep, don't run them.
At the time of this project, we use the latest Kubernetes version compatible with Minikube (`v1.28.3`), but this will probably be outdated relatively soon.
If you're trying to run this at some point in the future and are running into problems, you might want to use the same Kubernetes version we used for development.

```shell
# use this to reset your Minikube instance
minikube stop
minikube delete --all --purge

# create a new Minikube instance
minikube start --kubernetes-version=v1.28.3
```

### 3. Setting up Docker

On MacOS, Docker is running using a Linux VM. 
Minikube is running as a container inside that virtual machine.
Furthermore, that Minikube is itself running another instance of Docker. 
The Docker instance running inside Minikube is the relevant one for running Nuclio in this setup.
Run the following command to point your Docker CLI towards the Docker running inside Minikube.

```shell
eval $(minikube docker-env)
```

### 4. Build your local Nuclio version

The next step is to build Nuclio. Running this for the first time might (probably will) take some time. 
For me, this step took about 45 to 50 minutes using eduroam (bad choice!).
If you want to avoid frustration, make sure that you have a stable internet connection.
If you're connection is interrupted while building Nuclio, you get to start (almost) at the beginning.

I assume that this step is faster using Linux, as Docker does not run natively on MacOS, but it will probably still take a couple of minutes.
Also, the nexts builds will be faster (and you don't have to rebuild everything every time).

```shell
make build
```

Use `time make build` if you're interested in seeing how long this step takes on your machine.

### 5. Run a local insecure registry inside the cluster

Next up, we need a registry to push images to when we want to deploy functions in our cluster.
The Nuclio dashboard pushes them automatically for us, but we still need to set up the registry.
Note that this step will create and start a container inside the Minikube's Docker instance because of step 3.
Consequently, we have to run this in the same terminal as before (or run `eval $(minikube docker-env)` again).

```shell
docker run -d -p 5000:5000 --name function-registry registry:latest
```

### 6. Run Nuclio in your local cluster

Now, we want to run the Nuclio we just build inside the cluster.
To do so, we use Helm.
Depending on your machine, you might have to adjust the image tags.
You can use `docker image ls` to see them; it will likely be either `latest-arm64` or `latest-amd64`.

```shell
helm install nuclio \
    --set registry.pushPullUrl=localhost:5000 \
	--set controller.image.tag=latest-arm64 \
	--set dashboard.image.tag=latest-arm64 \
	--set dashboard.baseImagePullPolicy=Never \
	./hack/k8s/helm/nuclio/
```

### 7. Make the Nuclio dashboard accessible from outside the cluster

Run this command to access the Nuclio dashboard. 
On MacOS, it will only be accessible while this is running in a terminal somewhere—once you close that terminal, Nuclio is still running but you won't find it outside Minikube. 

```shell
kubectl port-forward $(kubectl get pod -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070
```

Now, use your browser and go to [http://127.0.0.1:8070/](http://127.0.0.1:8070/) or use nuctl to create and deploy some functions.

----

## Updating the Nuclio deployment after making changes

If you make changes to the Nuclio source code and want to run them, you have to rebuild (parts of) Nuclio.
For example, if you only made changes to the dashboard, run the following command.

```shell
make dashboard
```

Next, we have to update the Nuclio version inside the cluster using Helm.

```shell
# make sure you're in the root nuclio directory
helm upgrade nuclio \
    --set registry.pushPullUrl=localhost:5000 \
	--set controller.image.tag=latest-arm64 \
	--set dashboard.image.tag=latest-arm64 \
	--set dashboard.baseImagePullPolicy=Never \
	./hack/k8s/helm/nuclio/
```

Lastly, restart Nuclio somehow, for example, by restarting the Minikube VM.

```shell
minikube stop
minikube start
```

----

## Helpful commands

Here are some helpful commands in no particular order.

### Follow the registry's logs

```shell
minikube ssh -- docker logs -f function-registry
```

### Get the names of all images in the local registry

```shell
minikube ssh -- curl localhost:5000/v2/_catalog -v
```

### Follow the logs of the Nuclio dashboard (works the same way for individual functions)

```shell
# figure out what the pod is called
kubectl get pods -A

# follow the logs
kubectl logs pod/<DASHBOARD-POD-NAME> -f
```
