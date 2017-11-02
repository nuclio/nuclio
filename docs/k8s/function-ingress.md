# Invoking Functions By Name With Kubernetes Ingresses

If you followed the [getting started guide](getting-started.md), you invoked functions using their HTTP interface with `nuctl` and the playground. By default, each function deployed to Kubernetes declares a [Kubernetes service](https://kubernetes.io/docs/concepts/services-networking/service/) responsible for routing requests to the functions HTTP trigger port. It does this using a [NodePort](https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport) - a cluster-wide unique port assigned to the function.

This means that an underlying HTTP client called `http://<your cluster IP>:<some unique port>`. You can try this out yourself by first finding out the `NodePort` assigned to your function with `nuctl get function` (or with `kubectl get svc`) and using `curl` to send an HTTP request to this port.

In addition to configuring a service, nuclio will also create a [Kubernetes ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) for your function's HTTP trigger - with the path specified as `<function name>/latest`. However, without an ingress controller running in your cluster this will have no effect. An Ingress controller will listen for changed ingresses and re-configure a reverse proxy of some sort to route requests based on rules specified in the ingress.

## Setting Up An Ingress Controller
In this guide we'll set up [Træfik](https://docs.traefik.io/) though any Kubernetes ingress controller should work. You can head over to [Træfik's excellent documentation](https://docs.traefik.io/user-guide/kubernetes/) but assuming you don't want to use helm it just boils down to:

```bash
kubectl apply -f https://raw.githubusercontent.com/containous/traefik/master/examples/k8s/traefik-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/containous/traefik/master/examples/k8s/traefik-deployment.yaml
```

Check that the controller is up by running `kubectl --namespace=kube-system get pods` then run `kubectl describe service --namespace=kube-system traefik-ingress-service` to get the ingress `NodePort`:

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

Træfik's reverse proxy NodePort in this case is `30019` and we need to make sure to send all of our requests there. Let's deploy a function normally:

```bash
nuctl deploy -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry $(minikube ip):5000 helloworld --run-registry localhost:5000
```

And now invoke it by its path (specify your cluster IP if not using minikube):
```bash
curl $(minikube ip):<NodePort>/helloworld/latest
```

## Customizing Function Ingress
By default, functions will initialize the HTTP trigger and register `<function name>/latest`. However, we might want to add paths for functions to organize them in namespaces/groups or even choose through which domain our functions can be triggered. To do this, we can configure our HTTP trigger in the [function's configuration](/docs/configuring-a-function.md):

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

If our `helloworld` function were configured as such and assuming that Træfik's NodePort is 30019, it would be accessible through:
* `<cluster ip>:30019/helloworld/latest`
* `some.host.com:30019/helloworld/latest`
* `some.host.com:30019/first/path`
* `some.host.com:30019/second/path`
* `<cluster ip>:30019/wat`
* `some.host.com:30019/wat`

Note that since the `i1` explicitly specifies `some.host.com` as the `host` for the paths, they will _not_ be accessible through the cluster IP (i.e. `<cluster ip>:30019/first/path` will return 404).

## Deploying an Ingress Example

Let's try to put this into practice and deploy the [ingress example](/hack/examples/golang/ingress). The function.yaml is defined as:

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

And the handler as: 
```
func Ingress(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	return "Handler called", nil
}
```

Deploy it with nuctl (assuming minikube):
```bash
nuctl deploy -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/ingress/ingress.go --registry $(minikube ip):5000 helloworld --run-registry localhost:5000 --verbose
```

Invoke it with `nuctl`:
```bash
nuctl invoke ingress

> Response headers:
Server = nuclio
Date = Thu, 02 Nov 2017 02:11:32 GMT
Content-Type = text/plain; charset=utf-8
Content-Length = 14

> Response body:
Handler called
```

Now lets add my.host.com to our local `hosts` file so that it resolves to our cluster IP (assuming minikube):
```bash
echo "$(minikube ip) my.host.com" | sudo tee -a /etc/hosts
```

And now do some invocations with curl:
```
curl $(minikube ip):30019/ingress/latest (works)
curl my.host.com:30019/ingress/latest (works)

curl $(minikube ip):30019/first/path (works)
curl my.host.com:30019/first/path (works)

curl my.host.com:30019/first/from/host (works)
curl $(minikube ip):30019/first/from/host (404)
```
