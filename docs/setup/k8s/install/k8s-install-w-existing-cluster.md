# Using an Existing Kubernetes Cluster

This guide outlines the requirements for using an existing Kubernetes cluster that is capable of receiving nuclio functions.

## Kubernetes-cluster requirements
The only requirement for using an existing Kubernetes cluster with nuclio is that the cluster must support Custom Resource Definitions (CRDs), which were introduced in v1.7 of Kubernetes.

## Docker-registry requirement

To host your function images, you also need a Docker registry that will interface with nuclio and your cluster. If you don't want to litter [Docker Hub](https://docs.docker.com/docker-hub/) with your images, you'll need to spin up a docker registry to which you can push images and from which Kubernetes can pull images. For information on how to implement this, see the nuclio [Minikube](/docs/setup/k8s/install/k8s-install-minikube.md) and [kubeadm on Linux](/docs/setup/k8s/install/k8s-install-kubeadm-linux.md) Kubernetes installation guides. The Minikube guide explains how to run a registry container directly from Docker, whereas the `kubeadm` Linux guide outlines how to use a registry proxy and HostPort. Both methods allow for the `kubelets` "node agent" to pull images from `localhost:5000`, thus triggering the insecure registry logic without having to configure this per node.

## nuclio CLI note

The [nuclio CLI](/docs/reference/nuctl/nuctl.md) (`nuctl`) works wherever the [Kubernetes CLI](https://kubernetes.io/docs/user-guide/kubectl-overview/) (`kubectl`) works.

## What's next?

When you complete the steps in this guide, install nuclio on your cluster by following the instructions in the [Getting Started with nuclio on Kubernetes](/docs/setup/k8s/getting-started-k8s.md) guide.

