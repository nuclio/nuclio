# Private Docker Registries

During the build process, the builder will attempt to push the function image to a registry of your choice. Users who don't want to use their docker hub for their function images can instruct nuclio to push to a private docker registry and tell the platform (i.e. Kubernetes) to pull from the same location.

## Running a Private Insecure Docker Registry In Kubernetes

Bringing up an insecure private docker registry is trivial. However, attempting to pull from URLs which aren't explicitly defined as insecure in the docker daemon configuration will result in a pull error. Docker will, however, allow local registries (i.e. localhost / 127.0.0.1) to be insecure. This leaves us with two options.

### Option 1: Modify Docker Daemon Configuration on all Current and Future Nodes
In this scenario we can create a docker registry with a simple deployment and service:

```bash
kubectl apply -f <TODO>
```

And then configure all docker daemons in the cluster to allow insecure access to this service's URL.

### Option 2: Use HostPort
When configuring a service as `HostPort`, all services can access localhost:<HostPort> and reach the underlying pod. A [standard Kubernetes practice](https://github.com/kubernetes/kubernetes/tree/master/cluster/addons/registry) is to run a docker registry proxy with a `DaemonSet` on all nodes set up with a `HostPort`. Whenever kubelets access `localhost:<HostPort>` they hit their node's docker registry proxy which forwards the request to the actual docker registry instance. 

The [installation guide from scratch](install/linux.md) details how to apply this to your cluster. The unfortunate problem here is that `HostPort` does not work [reliably across CNIs](https://github.com/kubernetes/kubernetes/issues/23920). In fact, to make this work the installation guide applies a patch to Weave. Should you want to do this you would need to first download the [Weave patching script](https://github.com/nuclio/nuclio/blob/master/hack/k8s/scripts/install_cni_plugins) and run it on your cluster and then create the `HostPort` enabled registry:

```bash
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/registry.yaml
```

## Using External Private Registries (e.g. quay, ECR, GCR)
If you already have a private registry set up, nuclio is more than happy to push and pull images there. However, there are two crucial things that are (currently) out of the scope of nuclio:

1. Authentication to said registries - you need to make sure that the docker client underneath `nuctl` and `playground` is already logged in (it goes without saying that your kubelets also need to be authenticated)
2. In AWS ECR you need to create a repository for each function you create - this is not done automatically. In the future, nuclio can be made aware that it is pushing to ECR/GCR and with the proper API keys can create these repositories itself

To supply this to `nuctl`, you would just need to pass `--registry <your private registry URL>`. To supply this to playground, run the playground with `NUCLIO_PLAYGROUND_REGISTRY_URL` set to your private registry URL. 

## Registry vs Run Registry

Both `nuctl` and `playground` support different registry URLs for push and pull. If you specify only the "registry" - it is used for both push and pull. If you specify "registry" and "run registry", the former will be used for push and the latter for pull. For example, in `nuctl` you would pass `--registry <push registry> --run-registry <pull registry>` and in `playground` you would pass `NUCLIO_PLAYGROUND_REGISTRY_URL` and `NUCLIO_PLAYGROUND_RUN_REGISTRY_URL`. 
  