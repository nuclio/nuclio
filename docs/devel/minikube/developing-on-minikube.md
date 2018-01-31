# Developing with minikube
This guide will guide you through the process of deploying and running your locally built nuclio sources on a local Kubernetes cluster in your minikube VM. This is helpful when you're developing new functionality in the Kubernetes platform and need to test it against a real Kubernetes cluster.

## Working assumptions
This guide assumes that:
- You set up your minikube VM as described in the [minikube getting started guide](/docs/setup/k8s/install/k8s-install-minikube.md)
- You have previously deployed a _released_ nuclio version on top of it and interacted with it
- You have a working nuclio development environment and you're on a branch containing changes you made which pertain to the Kubernetes platform

## Get your local images onto minikube
When you install nuclio's services onto minikube (using `kubectl apply`), Kubernetes examines the given resource specification to determine which images to use for nuclio's controller and playground services. To get it to take your images, we must first push them onto the local Docker registry running inside the minikube VM. To do this:
- Make sure you've built Docker images with your changes (`make build`)
- Push them by running the script located at `hack/minikube/scripts/push_images.py`. Keep in mind the script assumes the local Docker registry to be listening on port 5000 of the minikube VM. It does the following:
   - Iterates over the existing nuclio Docker images on the host machine
   - For each such image:
     - Tags it locally as `$(minikube ip):5000/<image>` (i.e. `192.168.64.4:5000/processor:latest-amd64`)
     - Pushes it to the docker registry. Since the image's tag refers to a registry, it's pushed to the minikube's registry.
     - Untags it locally
     - _(in the minikube VM)_ Pulls the image, specifying the local Docker registry (i.e. `docker pull localhost:5000/processor:latest-amd64`)
     - _(in the minikube VM)_ Tags it with the `nuclio/` prefix (i.e. `nuclio/processor:latest-amd64`)
     - _(in the minikube VM)_ Untags the minikube-specific tag

This will make the latest versions of our locally-built images available from the Docker registry in the minikube VM.

## Deploy a custom version of the nuclio services
The `nuclio.yaml` resource specification that we feed `kubectl apply` with when deploying a released nuclio version always points to controller and playground images fixed to that version. In our case, we must use a modified version:
```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/development/hack/minikube/resources/devel/nuclio.yaml
```
It differs from the usual `nuclio.yaml` in that:
1) Controller/playground images are "latest", resulting in the images you pushed in the last step being used
2) Controller/playground images are never pulled from Docker Hub
3) Playground is told (via an environment variable) not to pull base images when deploying functions (it'll use the images you pushed)

You should now have a functional Kubernetes cluster using images built from your local changes, and can test against it to make sure they work as expected. Keep in mind when using a locally-built latest `nuctl`, to specify `--no-pull` such that the base images you pushed are used.
