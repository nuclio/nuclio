# Private Docker Registries

#### In this document

- [Overview](#overview)
- [Running a private insecure Docker registry in Kubernetes](#running-a-private-insecure-docker-registry-in-kubernetes)
- [Using an external private registry (such as Quay, ECR, or GCR)](#using-an-external-private-registry-such-as-quay-ecr-or-gcr)

## Overview

During the build process, the nuclio [builder](/README.md/#builder) attempts to push the function image to a registry of your choice. The default registry is your Docker Hub. If you don't want to deploy your functions to the Docker Hub, you can select one of the methods outlined in this guide to configure a private Docker registry to which nuclio will push images and from which the Kubernetes cluster will pull images.

## Running a private insecure Docker registry in Kubernetes

Bringing up an insecure private Docker registry in the Kubernetes cluster is trivial. However, attempting to pull from a registry URL that isn't explicitly defined as insecure in the Docker daemon configuration will result in a pull error. But Docker does allow local registries (i.e., `localhost` / `127.0.0.1`) to be insecure. This leaves you with two options.

### Option 1: Configure the Docker daemon on each node to use a local registry

You can create a local Docker registry with a simple deployment service:

```sh
kubectl apply -f <deployment-service configuration>
```

And then, configure the Docker daemons for all cluster nodes to allow insecure access to the URL of this service, thus modifying the daemon configuration to use your local registry. Note that you'll need to repeat this process for any nodes that you add to the cluster in the future.

### Option 2: Use HostPort

When configuring a service as a HostPort, all services can access `localhost:<HostPort>` and reach the underlying pod. A [standard Kubernetes practice](https://github.com/kubernetes/kubernetes/tree/master/cluster/addons/registry) is to run a Docker registry proxy with a `DaemonSet` on all nodes set up with a HostPort. Whenever a `kubelets` "node agent" accesses `localhost:<HostPort>`, it hits its node's Docker registry proxy, which forwards the request to the actual Docker registry instance. 

The [Installing Kubernetes with kubeadm on Linux Ubuntu](/docs/setup/k8s/install/k8s-install-kubeadm-linux.md) guide explains how to apply this to your cluster. The unfortunate problem with this method is that HostPort does not work reliably across Container Network Interfaces (CNIs) (see https://github.com/kubernetes/kubernetes/issues/23920). In fact, to support this method, the installation guide instructs you to install a patch plugin for the Weave CNI. Therefore, this method currently requires that you first download the [Weave patch script](https://github.com/nuclio/nuclio/blob/master/hack/k8s/scripts/install_cni_plugins) and run it on your Kubernetes cluster, and then create the HostPort enabled registry:

```sh
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/registry.yaml
```

## Using an external private registry (such as Quay, ECR, or GCR)

If you already have a private registry set up, you can configure nuclio so that this registry will be used for pushing and pulling images. However, you should be aware that the following crucial requirements are (currently) out of nuclio's scope:

1.  Authentication by the private registry - you need to ensure that the Docker client, underneath the nuclio CLI (`nuctl`) and playground, is already logged into the registry. Note that your `kubelets` instance also needs to be authenticated with the registry.

2.  AWS ECR per-function repository - when using registries in the AWS Amazon Elastic Container Registry ([ECR](https://aws.amazon.com/ecr/)), you need to create a repository for each function that you define. This is not done automatically. Future nuclio plans include support for automatic creation of such repositories by nuclio, based on API configurations.

To configure nuclio to use a private registry, do the following:

- When using the `nuctl` CLI, add the `--registry` flag (`--registry <private-registry URL>`).

    By default, the configured registry URL will be used for both pushing and pulling images. However, you can add the `--run-registry` flag (`--run-registry <private registry URL>`) to use a different URL (for the same registry) for pulling images, in which case the configured `--registry` URL will be used only for pushing images.

- When using the playground, set the `NUCLIO_PLAYGROUND_REGISTRY_URL` environment variable to your private registry URL (`NUCLIO_PLAYGROUND_REGISTRY_URL=<private-registry URL>`).

    By default, the configured registry URL will be used for both pushing and pulling images. However, you can set the `NUCLIO_PLAYGROUND_RUN_REGISTRY_URL` environment variable (`NUCLIO_PLAYGROUND_RUN_REGISTRY_URL=<private registry URL>`) to use a different URL (for the same registry) for pulling images, in which case the configured `NUCLIO_PLAYGROUND_REGISTRY_URL` URL will be used only for pushing images.

