# Installing Kubernetes with kubeadm on Linux Ubuntu

Follow this guide to set up, "from scratch", a Kubernetes cluster that is capable of receiving nuclio functions, on a Linux Ubuntu server. The installation includes the following components:

- A vanilla Kubernetes installation
- The Weave Container Network Interface (CNI), and a plugin to support HostPort
- A private Docker registry and a related proxy

## Prerequisites

Ensure that the following prerequisites are met:

- You have an Ubuntu v16.04 server (the master installation machine) with the TCP ports 6443 and 31276 open. You should also open a range of ports for invoking functions over HTTP, within the range of 30000-32767.
- [A supported version of Go](/docs/setup/k8s/getting-started-k8s.md#go-supported-version) is installed on the master.

## Master installation steps

Perform the following steps on your Ubuntu server (the master machine):

1.  **Clone the nuclio GitHub repo** to your Go path (`$GOPATH`) by running the following command:

    ```sh
    git clone https://github.com/nuclio/nuclio.git $GOPATH/src/github.com/nuclio/nuclio
    ```

2.  **Install Docker** by running the following command. This is a prerequisite to the subsequent steps:

    ```sh
    $GOPATH/src/github.com/nuclio/nuclio/hack/k8s/scripts/install_docker
    ```

    Log out and log back in, and make sure to re-set `$GOPATH`. Run the following command to verify that Docker can be run without admin privileges (i.e., without `sudo`):

    ```sh
    docker run hello-world
    ```

3.  **Install required Kubernetes setup tools**: run the following command to install the [`kubectl`](https://kubernetes.io/docs/user-guide/kubectl-overview/) CLI, [`kubelet`](https://kubernetes.io/docs/reference/generated/kubelet/) "node agent", and [`kubeadm`](https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/) setup tool:

    ```sh
    $GOPATH/src/github.com/nuclio/nuclio/hack/k8s/scripts/install_kubeadm
    ```
4.  **Create a Kubernetes cluster**: run the following command to instruct `kubeadm` to create a cluster with the Weave CNI. Replace the `<external IP address>` placeholder with the external IP address of your installation machine, so that the certificate that `kubeadm` creates will be valid also for this machine. This will allow you to run `kubectl` remotely without running an insecure proxy:

    ```sh
    $GOPATH/src/github.com/nuclio/nuclio/hack/k8s/scripts/create_cluster <external IP address>
    ```

    This command can be run whenever you want a fresh cluster. However, for the first invocation you must also install a CNI plugin that fixes issues with HostPort. This is true as of 15 July 2017. In the future, this might be part of the default installation. For more information on this issue, see https://github.com/weaveworks/weave/issues/3016. Run the following command to install the plugin:

    ```sh
    $GOPATH/src/github.com/nuclio/nuclio/hack/k8s/scripts/install_cni_plugins
    ```

## Local installation steps

You're done running commands on the master. The following commands can be run on a local machine on which `kubectl` is installed:

1.  **Configure access to the cluster:** copy **~/kube/config** (your [_kubeconfig_](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) file) from the master node to **~/kube/config** on the local machine. Edit the file to change the IP address under `server` to the external IP address of your machine; (don't modify the `port` configuration). Then, run the following `kubectl` command to verify your configuration:
and test `kubectl`

    ```sh
    kubectl get pods --all-namespaces
    ```

2.  **Prepare the function-deployment infrastructure** by running the following command:

    ```sh
    cd $GOPATH/src/github.com/nuclio/nuclio/hack/k8s/resources && kubectl create -f default-cluster-admin.yaml,registry.yaml && cd -
    ```

    This commands creates the following:

    - **A nuclio deployment controller**, which will listen for function changes and ensure that the function deployments are up to speed.

    - **A Docker registry and a deployment proxy**, which will allow you to push function images to an insecure Docker registry on your cluster (at port 31276) rather than to [Docker Hub](https://docs.docker.com/docker-hub/) or to a remote private Docker registry.

      > **Note:** You'll need to configure `<cluster IP>:31276` as an insecure registry in your local Docker daemon. This will allow Kubernetes to pull these images from `localhost:5000`, using the registry proxy. Because the cluster daemon treats `localhost` as secure, you don't need configure the cluster daemon.

    - **A hole in the role-based access control (RBAC)**, which provides full access permissions to resources in the default namespace. In the future, this will be more fine grained.

## What's next?

When you complete the steps in this guide, install nuclio on your cluster by following the instructions in the [Getting Started with nuclio on Kubernetes](/docs/setup/k8s/getting-started-k8s.md) guide.

