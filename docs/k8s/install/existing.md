# Using an existing Kubernetes cluster

There are no special requirements from an existing Kubernetes cluster other than it supported Custom Resource Definitions (1.7+). However, you will need a docker registry to host your function images.

If you don't want to litter docker hub with your images, you'll need to spin up a docker registry you can push to and Kubernetes can pull from. You can both the [minikube](minikube.md) and [linux](linux.md) guides for ways to do this. The former simply runs a registry container directly from docker whereas the latter uses a registry proxy and HostPort. Both allow for the kubelets to pull from localhost:5000, thus triggering the insecure registry logic without having to configure this per node.

The CLI (`nuctl`) will work wherever `kubectl` works.

Once that completes, you can resume the [getting started guide](/docs/k8s/getting-started.md) to install nuclio on this cluster.