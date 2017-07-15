This document will guide you through setting up a Kubernetes cluster capable of receiving nuclio functions. On top of vanilla kubernetes you'll install:
* Weave CNI + a plugin to support `HostPort`
* A private docker registry and a proxy
* The nuclio controller, which listens for changes on function custom resources and applies that to deployments

This guide assumes Ubuntu 16.04 server. Start by cloning nuclio to your GOPATH (make sure you have one first):

```
git clone https://github.com/nuclio/nuclio.git $GOPATH/src/github.com/nuclio/nuclio
```

### Installing kubeadm

Install docker, a prerequisite to everything:
```
$GOPATH/src/github.com/nuclio/nuclio/hack/k8s/scripts/install_docker
```

Log out and log back in (make sure to re-set $GOPATH). Verify docker works without sudo by running:
```
docker run hello-world
```

Now install kubectl + kubelet + kubeadm:
```
$GOPATH/src/github.com/nuclio/nuclio/hack/k8s/scripts/install_kubeadm
```

### Create a Kubernetes cluster

Tell `kubeadm` to create a cluster for us with the Weave CNI. You must specify the external IP address of the machine so that the certificate kubeadm creates will be valid for it as well. This will allow you to run kubectl remotely without running an insecure proxy:
```
$GOPATH/src/github.com/nuclio/nuclio/hack/k8s/scripts/create_cluster <external IP address>
```

The above command can be run whenever you want a fresh cluster. However, for the first invocation you must also install a CNI plugin that fixes issues with "HostPort". This is true as of 15th of July 2017 - it may be part of the default install (more about this issue here: https://github.com/weaveworks/weave/issues/3016).

```
$GOPATH/src/github.com/nuclio/nuclio/hack/k8s/scripts/install_cni_plugins
```

We're done running commands on the master, now we move on to a local machine with kubectl installed. 

Copy `~/kube/config` from the master node to `~/kube/config`, change the IP address under `server` to the external IP address (leave port as is) and test kubectl:

```
kubectl get pods --all-namespaces
```

Finally, create a docker registry, a docker registry proxy and grant the default namespace complete access to everything via RBAC:
```
cd $GOPATH/src/github.com/nuclio/nuclio/hack/k8s/resources && kubectl create -f full-access-role.yaml,registry.yaml && cd -
```

### Build / deploy a controller
TODO