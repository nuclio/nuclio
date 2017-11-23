# Invoking Functions by Name with a Kubernetes Ingress

#### In This Document

- [Overview](#overview)
- [Setting Up an Ingress Controller](#setting-up-an-ingress-controller)
- [Customizing Function Ingress](#customizing-function-ingress)
- [Deploying an Ingress Example](#deploying-an-ingress-example)

## Overview

If you followed the [Getting Started with nuclio on Kubernetes](getting-started.md) guide, you invoked functions using their HTTP interface with `nuctl` and the nuclio playground. By default, each function deployed to Kubernetes declares a [Kubernetes service](https://kubernetes.io/docs/concepts/services-networking/service/) that is responsible for routing requests to the functions' HTTP trigger port. It does this using a [NodePort](https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport), which is a unique cluster-wide port that is assigned to the function.

This means that an underlying HTTP client calls `http://<your cluster IP>:<some unique port>`. You can try this out yourself: first, find out the NodePort assigned to your function, by using the `nuctl get function` command of the `nuctl` CLI or the `kubectl get svc` command of the Kubernetes CLI. Then, use Curl to send an HTTP request to this port.

In addition to configuring a service, nuclio creates a [Kubernetes ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) for your function's HTTP trigger, with the path specified as `<function name>/latest`. However, without an ingress controller running in your cluster, this will have no effect. An Ingress controller will listen for changed ingresses and reconfigure some type of reverse proxy to route requests based on rules specified in the ingress.

## Setting Up an Ingress Controller

In this guide, you will set up a [Træfik](https://docs.traefik.io/) controller, but any type of Kubernetes ingress controller should work. You can read [Træfik's excellent documentation](https://docs.traefik.io/user-guide/kubernetes/), but for the purposes of this guide you can simply run the following commands to set up the controller:

```bash
kubectl apply -f https://raw.githubusercontent.com/containous/traefik/master/examples/k8s/traefik-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/containous/traefik/master/examples/k8s/traefik-deployment.yaml
```

Verify that the controller is up by running by running the `kubectl --namespace=kube-system get pods` command, and then run the `kubectl describe service --namespace=kube-system traefik-ingress-service` command to get the ingress NodePort. Following is a sample output for NodePort 30019:

```bash
...
Port:                     web  80/TCP
TargetPort:               80/TCP
NodePort:                 web  30019/TCP
Endpoints:                172.17.0.8:80
Port:                     admin  8080/TCP
TargetPort:               8080/TCP
...
```

> **Note:** You must ensure that all your requests are sent to the returned NodePort.

Run the following command to deploy the sample `helloworld` function; (the command assumes the use of Minikube):
```bash
nuctl deploy -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry $(minikube ip):5000 helloworld --run-registry localhost:5000
```

And now, invoke the function by its path.
Replace `<NodePort>` with the NodePort of your ingress controller, and replace `${minikube ip)` with your cluster IP if you are not using Minikube:
```bash
curl $(minikube ip):<NodePort>/helloworld/latest
```

For example, for NodePort 30019, run this command:
```bash
curl $(minikube ip):30019/helloworld/latest
```

## Customizing Function Ingress

By default, functions initialize the HTTP trigger and register `<function name>/latest`. However, you might want to add paths for functions to organize them in namespaces/groups, or even choose through which domain your functions can be triggered. To do this, you can configure your HTTP trigger in the [function's configuration](/docs/configuring-a-function.md). For example:

```yaml
  ...
  triggers:
    http:
      maxWorkers: 4
      kind: "http"
      attributes:
        ingresses:
          i1:

            # this assumes that some.host.com points to <cluster ip>
            host: "some.host.com"
            paths:
            - "/first/path"
            - "/second/path"
          i2:
            paths:
            - "/wat"
```

If your `helloworld` function was configured in this way, and assuming that Træfik's NodePort is 30019, the function would be accessible through any of the following URLs:

- `<cluster ip>:30019/helloworld/latest`
- `some.host.com:30019/helloworld/latest`
- `some.host.com:30019/first/path`
- `some.host.com:30019/second/path`
- `<cluster ip>:30019/wat`
- `some.host.com:30019/wat`

Note that since the `i1` configuration explicitly specifies `some.host.com` as the `host` for the paths, the function will _not_ be accessible through the cluster IP; i.e., `<cluster ip>:30019/first/path` will return a `404` error.

## Deploying an Ingress Example

Let's put this into practice and deploy the [ingress example](/hack/examples/golang/ingress/ingress.go). This is the **function.yaml** file for the example:

```yaml
apiVersion: "nuclio.io/v1"
kind: "Function"
spec:
  runtime: "golang"
  triggers:
    http:
      maxWorkers: 8
      kind: http
      attributes:
        ingresses:
          first:
            paths:
            - /first/path
            - /second/path
          second:
            host: my.host.com
            paths:
            - /first/from/host
```

And this is the definition of the `Ingress` handler function: 
```golang
func Ingress(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	return "Handler called", nil
}
```

### Deploy the Function

Deploy the function with the `nuctl` CLI. If you did not use Minikube, replace `$(minikube ip):5000` in the following command with your cluster IP:
```bash
nuctl deploy -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/ingress/ingress.go --registry $(minikube ip):5000 ingress --run-registry localhost:5000 --verbose
```

Behind the scenes, `nuctl` populates a function CR, which is picked up by the nuclio `controller`. The `controller` iterates through all the triggers and looks for the required ingresses. For each ingress, the controller creates a Kubernetes Ingress object, which triggers the Træfik ingress controller to reconfigure the reverse proxy. Following are sample `controller` logs:

```
controller.functiondep (D) Adding ingress {"function": "helloworld", "host": "", "paths": ["/helloworld/latest"]}
controller.functiondep (D) Adding ingress {"function": "helloworld", "host": "my.host.com", "paths": ["/first/from/host"]}
controller.functiondep (D) Adding ingress {"function": "helloworld", "host": "", "paths": ["/first/path", "/second/path"]
```

### Invoke the Function with nuctl

Invoke the function with `nuctl`, which will use the configured NodePort:
```bash
nuctl invoke ingress
```
Following is a sample output for this command:
```bash
> Response headers:
Server = nuclio
Date = Thu, 02 Nov 2017 02:11:32 GMT
Content-Type = text/plain; charset=utf-8
Content-Length = 14

> Response body:
Handler called
```

### Configure a Custom Host

Add `my.host.com` to your local **/etc/hosts** file so that it resolves to your cluster IP. The following command assumes the use of Minikube:
```bash
echo "$(minikube ip) my.host.com" | sudo tee -a /etc/hosts
```

### Invoke the Function with Curl

Now, do some invocations with Curl. The following examples assume the use of Minikube (except were your configured host is used) and NodePort 30019.

> **Note:** The parenthesized "works" and error indications at the end of each line signify the expected outcome and are not part of the command.

```bash
curl $(minikube ip):30019/ingress/latest (works)
curl my.host.com:30019/ingress/latest (works)

curl $(minikube ip):30019/first/path (works)
curl my.host.com:30019/first/path (works)

curl my.host.com:30019/first/from/host (works)
curl $(minikube ip):30019/first/from/host (404 error)
```

