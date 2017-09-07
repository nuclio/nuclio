# Installing using Vagrant

## Prerequisites

Please make sure you have the following installed on your machine

- [VirtualBox](https://www.virtualbox.org/)
- [Vagrant](https://www.vagrantup.com/)

Make sure `vagrant` is working by running `vagrant version`

### MacOS: Installing prerequisites with Brew

```bash
$ brew install caskroom/cask/virtualbox
$ brew install caskroom/cask/vagrant
```

## Installing Kubernetes in Vagrant

From current folder (`$GOPATH/src/github.com/nuclio/hack/k8s/vagrant`) run:

```bash
$ vagrant up
```

This will start an Ubuntu 16.04 VM and run each of the [required steps](../../../docs/k8s/README.md) to have a Kubernetes cluster running with Nuclio.

### Cluster Defaults

- Host IP: `10.100.100.10`
- Docker Registry: `10.100.100.10:31276`
- GOPATH: `/opt/nuclio`

## Accessing the vagrant machine

From current folder (`$GOPATH/src/github.com/nuclio/hack/k8s/vagrant`) run:

```bash
$ vagrant ssh
```

You can make sure the cluster is fully functional by running:

```bash
ubuntu@k8s:~$ kubectl get pods --all-namespaces
```
Output should be similar to:
```bash
  NAMESPACE     NAME                                 READY     STATUS    RESTARTS   AGE
  kube-system   etcd-k8s                             1/1       Running   0          8m
  kube-system   kube-apiserver-k8s                   1/1       Running   0          8m
  kube-system   kube-controller-manager-k8s          1/1       Running   0          8m
  kube-system   kube-dns-2425271678-nw37t            3/3       Running   0          8m
  kube-system   kube-proxy-44pr8                     1/1       Running   0          8m
  kube-system   kube-registry-proxy-d5mgc            1/1       Running   0          8m
  kube-system   kube-registry-v0-nswz4               1/1       Running   0          8m
  kube-system   kube-scheduler-k8s                   1/1       Running   0          8m
  kube-system   weave-net-jc601                      2/2       Running   0          8m
```