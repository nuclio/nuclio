# Installing Kubernetes Locally with Minikube

Follow this guide to use [Minikube](https://github.com/kubernetes/minikube/) to set up a single-node Kubernetes cluster that is capable of running nuclio functions, inside a virtual machine (VM) on a local computer.

> **Note:** This is alpha-level documentation. Your feedback is welcome. Use the nuclio [GitHub issues](https://github.com/nuclio/nuclio/issues) or the [#docs](https://nuclio-io.slack.com/messages/C83US1NDP/) Slack channel to report any issues or offer improvements.<br/>
> For more information about Minikube, see the [Kubernetes documentation](https://kubernetes.io/docs/getting-started-guides/minikube/).

## OS X Installation

Follow these steps to use Minikube to install a Kubernetes cluster on OS X:

### Prerequisites

Ensure that the following components are installed on your installation machine:

- [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)
- [xhyve driver](https://github.com/kubernetes/minikube/blob/master/docs/drivers.md#xhyve-driver)

### Installation

1.  Start Minikube as you normally would:

    ```sh
    minikube start --vm-driver=xhyve
    ```

2.  Bring up a docker registry inside Minikube. You'll later push your functions to this registry:

    ```sh
    minikube ssh -- docker run -d -p 5000:5000 registry:2
    ```

3.  Finally, bind the default service account to the cluster-admin role; in the future, the hole punched into the role-based access control (RBAC) will be smaller:

    ```sh
    kubectl apply -f https://raw.githubusercontent.com/nuclio/nuclio/master/hack/k8s/resources/default-cluster-admin.yaml
    ```

## What's next?

When you complete the steps in this guide, install nuclio on your cluster by following the instructions in the [Getting Started with nuclio on Kubernetes](/docs/setup/k8s/getting-started-k8s.md) guide.

