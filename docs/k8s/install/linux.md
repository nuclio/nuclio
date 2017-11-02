# Installing Kubernetes from scratch with kubeadm on Ubuntu

This document will guide you through setting up a Kubernetes cluster capable of receiving nuclio functions. On top of vanilla kubernetes you'll install:
* Weave CNI + a plugin to support `HostPort`
* A private docker registry and a proxy

This guide assumes Ubuntu 16.04 server with the TCP ports 6443 and 31276 open (you should also open range on which functions can be invoked over HTTP in the range of 30000-32767). Start by cloning nuclio to your GOPATH (make sure you have one first):

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

The above command can be run whenever you want a fresh cluster. However, for the first invocation you must also install a CNI plugin that fixes issues with "HostPort". This is true as of 15th of July 2017 - it may be part of the default install in the future (more about this issue here: https://github.com/weaveworks/weave/issues/3016).

```
$GOPATH/src/github.com/nuclio/nuclio/hack/k8s/scripts/install_cni_plugins
```

We're done running commands on the master, now we move on to a local machine with kubectl installed.

Copy `~/kube/config` from the master node to `~/kube/config`, change the IP address under `server` to the external IP address (leave port as is) and test kubectl:

```
kubectl get pods --all-namespaces
```

In the final step we'll create the following:
1. A nuclio controller deployment: this will listen for function changes and make sure the function deployments are up to speed (see # for more details)
2. A docker registry + proxy deployment: this will allow you to push function images to an insecure docker registry on your cluster (@ port 31276) rather than docker hub or some private docker registry somewhere. Note that you'll need to configure <cluster IP>:31276 as an insecure registry in your local docker daemon. Kubernetes will be able to pull these images from localhost:5000 thanks to the registry proxy, so we're excused from the need to configure the cluster daemon seeing how it treats localhost as secure
3. A hole in the RBAC allowing resources in the default namespace to do everything. In the future this will be more fine grained

```
cd $GOPATH/src/github.com/nuclio/nuclio/hack/k8s/resources && kubectl create -f default-cluster-admin.yaml,registry.yaml && cd -
```

Once that completes, you can resume the [getting started guide](/docs/k8s/getting-started.md) to install nuclio on this cluster.
