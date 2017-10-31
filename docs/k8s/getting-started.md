# Getting Started With nuclio On Kubernetes

To start deploying functions we'll need a remote Kubernetes **1.7+** cluster (nuclio uses CRDs, introduced in 1.7) which we can prepare in one of three ways:

1. [On a local VM with minikube](install/minikube.md) - recommended for beginners
2. [From scratch with kubeadm on Ubuntu](install/linux.md)
3. [On an existing Kubernetes cluster](install/existing.md)

For the sake of simplicity, this guide will assume you're using minikube - just replace `$(minikube ip)` with your cluster IP in the commands listed here if you're not using minikube.

With a functioning Kuberenetes cluster, docker registry and a working local kubectl, we can go ahead and install the nuclio services on the cluster:

```bash
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/controller.yaml
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/playground.yaml
```

Use `kubectl get pods` to verify both controller and playground have a status of `Running`.

#### Deploying a Function With The nuclio Playground

Browse to `http://$(minikube ip):32050` - you should be greeted by the nuclio playground. Choose one of the built in examples and click deploy. The first build will populate the local docker cache with base images and such, so it might take a while depending on your network. Once the function has been deployed, you can invoke it with a body by clicking "Invoke".

#### Deploying a Function With nuctl, The nuclio Command Line Tool

First, make sure you have Golang 1.8+ (https://golang.org/doc/install) and Docker (https://docs.docker.com/engine/installation). Create a Go workspace (e.g. in `~/nuclio`):

```bash
export GOPATH=~/nuclio && mkdir -p $GOPATH
```

Now build nuctl, the nuclio command line tool and add `$GOPATH/bin` to path for this session:
```bash
go get -u github.com/nuclio/nuclio/cmd/nuctl
PATH=$PATH:$GOPATH/bin
```

Before docker images can be pushed to our built in registry, we need to add our integrated docker registry address (e.g. `$(minikube ip):5000` if you're using minikube) to the list of insecure registries. If you're using Docker for Mac you can find this under `Preferences -> Daemon`.

Deploy the Golang hello world example (you can add `--verbose` if you want to peek under the hood):
```bash
nuctl deploy -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry $(minikube ip):5000 helloworld --run-registry localhost:5000
```

And finally execute it:
```bash
nuctl invoke helloworld
```

Further reading:
1. [Configuring a function](/docs/configuring-a-function.md)
2. [Invoking functions by name with an ingress](function-ingress.md)
3. [More function examples](/hack/examples/README.md)
