# Getting started

To start deploying functions we'll need a remote Kubernetes **1.7+** cluster (nuclio uses CRDs, introduced in 1.7) which we can install in one of two ways:

1. [On a local VM with minikube](k8s/install/minikube.md) - recommended
2. [From scratch with kubeadm on Ubuntu](k8s/install/linux.md)

If you already have a Kubernetes cluster, you'll need to run a docker registry on it so that nuclio can push functions to it. In the minikube guide we do this with vanilla docker, whereas with kubeadm/scratch we employ a docker registry proxy. As long as you can push images to it and Kubernetes can pull images from it - you should be good.

With a functioning kuberenetes cluster (with built-in docker registry) and a working kubectl, we can go ahead and install the nuclio services on the cluster:

```
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/controller.yaml
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/playground.yaml
```

Use `kubectl get pods` to verify both controller and playground have a status of `Running`.

#### Deploying a function with the nuclio playground

Browse to `http://<cluster-ip>:32050` - you should be greeted by the nuclio playground. Choose one of the built in examples and click deploy. The first build will populate the local docker cache with base images and such, so it might take a while depending on your network. Once the function has been deployed, you can invoke it with a body by clicking "Invoke".

#### Deploying a function with nuctl, the nuclio command line tool

First, make sure you have Golang 1.8+ (https://golang.org/doc/install) and Docker (https://docs.docker.com/engine/installation). Create a Go workspace (e.g. in `~/nuclio`):

```
export GOPATH=~/nuclio && mkdir -p $GOPATH
```

Now build nuctl, the nuclio command line tool and add `$GOPATH/bin` to path for this session:
```
go get -u github.com/nuclio/nuclio/cmd/nuctl
PATH=$PATH:$GOPATH/bin
```

Before docker images can be pushed to our built in registry, we need to add our integrated docker registry address (e.g. `$(minikube ip):5000` if you're using minikube) to the list of insecure registries. If you're using Docker for Mac you can find this under `Preferences -> Daemon`.

Deploy the Golang hello world example (you can add `--verbose` if you want to peek under the hood):
```
nuctl deploy -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry [registry address] helloworld --run-registry localhost:5000
```

And finally execute it:
```
nuctl invoke helloworld
```

You can now poke around the (examples directory)[/hack/examples] to get some inspiration or move on to some more advanced reading:

1. [More examples](/hack/examples)
2. TODO
