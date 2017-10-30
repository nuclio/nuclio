# Getting started

To start deploying functions we'll need a remote Kubernetes **1.7+** cluster (nuclio uses CRDs, introduced in 1.7) which we can install in one of two ways:

1. [On a local VM with minikube](k8s/install/minikube.md) - recommended
2. [From scratch with kubeadm on Ubuntu](k8s/install/linux.md)

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

Before docker images can be pushed to our built in registry, we need to add `<cluster-ip>:31276` (e.g. `10.100.100.10:31276` if you're using Vagrant) to the list of insecure registries. If you're using Docker for Mac you can find this under `Preferences -> Daemon`.

Deploy the hello world example:
```
nuctl deploy -p  --registry [registry address] helloworld --run-registry localhost:5000
```

If you're using `minikube`, the registry address is `$(minikube ip):5000`. If you used `kubeadm`, the registry address is `<cluster-ip>:31276`.

And finally execute it:
```
nuctl invoke helloworld
```
