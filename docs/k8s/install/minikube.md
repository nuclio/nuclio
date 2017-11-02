# Installing Kubernetes with minikube

Note: This is alpha level documentation - if you encounter any issues, feedback is welcomed.

## OSX

### Prerequisites

Please make sure you have the following installed on your machine:
- [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)
- [xhyve driver](https://github.com/kubernetes/minikube/blob/master/docs/drivers.md#xhyve-driver)

### Installation

Start minikube as you normally would:
```
minikube start --vm-driver=xhyve
```

Bring up a docker registry inside minikube so that we can push our functions to it:
```
minikube ssh -- docker run -d -p 5000:5000 registry:2
```

Finally, bind the default service account to the cluster-admin role (in the future, the hole punched into RBAC will be smaller):
```
kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/default-cluster-admin.yaml
```

Once that completes, you can resume the [getting started guide](/docs/k8s/getting-started.md) to install nuclio on this cluster.
