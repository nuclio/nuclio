# Developing with Minikube

This guide will guide you through the process of deploying and running your locally built Nuclio sources on a local Kubernetes cluster in your Minikube VM. This is helpful when you're developing new functionality in the Kubernetes platform and need to test it against a real Kubernetes cluster.

## Working assumptions

This guide assumes that:
- You set up your Minikube VM as described in the [Minikube getting started guide](../../setup/minikube/getting-started-minikube.md)
- You have previously deployed a _released_ Nuclio version on top of it and interacted with it
- You have a working Nuclio development environment, and you're on a branch containing changes you made which pertain to the Kubernetes platform

## Get your local images onto Minikube

When you install Nuclio's services onto Minikube (using the [Helm chart](https://github.com/nuclio/nuclio/tree/development/hack/k8s/helm/nuclio)), Kubernetes examines the given resource specification to determine which images to use for Nuclio's controller and dashboard services. To get it to take your images, we must first push them onto the local Docker registry running inside the Minikube VM. To do this:
- Make sure you've built container images with your changes (`make build`)
- Push them by running the script located at `hack/scripts/minikube/push_images.py`. Keep in mind the script assumes the local Docker registry to be listening on port 5000 of the Minikube VM. It does the following:
   - Iterates over the existing Nuclio container images on the host machine
   - For each such image:
     - Tags it locally as `$(minikube ip):5000/<image>` (i.e., `192.168.64.4:5000/processor:latest-amd64`)
     - Pushes it to the Docker registry. Since the image's tag refers to a registry, it's pushed to the Minikube registry.
     - Untags it locally
     - _(in the Minikube VM)_ Pulls the image, specifying the local Docker registry (i.e., `docker pull localhost:5000/processor:latest-amd64`)
     - _(in the Minikube VM)_ Tags it with the `nuclio/` prefix (i.e., `nuclio/processor:latest-amd64`)
     - _(in the Minikube VM)_ Untags the Minikube-specific tag

This will make the latest versions of our locally-built images available from the Docker registry in the Minikube VM.

## Deploy a custom version of the Nuclio services

The usual [Nuclio Helm chart](https://github.com/nuclio/nuclio/tree/development/hack/k8s/helm/nuclio) always points to controller and dashboard images fixed to some version,
but in our case, we must use a modified version.
From the project root run the following:
```sh
helm install nuclio \
    --set registry.pushPullUrl=localhost:5000 \
    --set controller.image.tag=latest-amd64 \
    --set dashboard.image.tag=latest-amd64 \
    --set dashboard.baseImagePullPolicy=Never \
	./hack/k8s/helm/nuclio/
```
It differs from the usual upstream chart in that:
1) Controller/dashboard images are "latest", resulting in the images you pushed in the last step being used
2) Controller/dashboard images are never pulled from Docker Hub
3) Dashboard is told (via an environment variable) not to pull base images when deploying functions (it'll use the images you pushed)
4) It uses the helm chart from a local copy - giving you additional ability to test any local changes to the nuclio chart

You should now have a functional Kubernetes cluster using images built from your local changes, and can test against it to make sure they work as expected. Keep in mind when using a locally-built latest `nuctl`, to specify `--no-pull` such that the base images you pushed are used.

