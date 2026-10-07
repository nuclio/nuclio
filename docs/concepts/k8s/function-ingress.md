# Invoking Functions by Name with a Kubernetes Ingress

## In this document

- [Overview](#overview)
- [Setting up an ingress controller](#setting-up-an-ingress-controller)
- [Customizing function ingress](#customizing-function-ingress)
- [Deploying an ingress example](#deploying-an-ingress-example)

## Overview

If you followed the [Getting Started with Nuclio on Kubernetes](../../setup/k8s/getting-started-k8s.md) or [Getting Started with Nuclio on Google Kubernetes Engine (GKE)](../../setup/gke/getting-started-gke.md) guide, you invoked functions using their HTTP interface with `nuctl` and the Nuclio dashboard.
By default, each function deployed to Kubernetes declares a [Kubernetes service](https://kubernetes.io/docs/concepts/services-networking/service/) that is responsible for routing requests to the functions' HTTP trigger port. On Kubernetes, this port is served by the auth-proxy sidecar, which authenticates the request before forwarding it to the processor. The authentication mode is controlled by the `authenticationMode` attribute of the function's HTTP trigger (see [HTTP trigger reference](../../reference/triggers/http.md)).
To invoke the function externally, using `nuctl`, you probably exposed your function by using a [NodePort](https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport), which is a unique cluster-wide port that is assigned to the function.

This means that if your function's HTTP trigger is configured with a `NodePort`, any underlying HTTP client can call `http://<your cluster IP>:<some unique port>` to reach it.
You can try this out yourself: first, find out the NodePort assigned to your function, by using the `nuctl get function` command of the `nuctl` CLI or the `kubectl get svc` command of the Kubernetes CLI. Then, use `curl` to send an HTTP request to this port.

In addition to configuring a service, Nuclio can create a [Kubernetes ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) for your function's HTTP trigger, with the path specified as `<function name>/latest`.
However, without an ingress controller running on your cluster, this will have no effect. An Ingress controller will listen for changed ingresses and reconfigure some type of reverse proxy to route requests based on rules specified in the ingress resource.

## Setting up an ingress controller

> **Warning: do not use `ingress-nginx`.** The Kubernetes project has [retired Ingress NGINX](https://www.kubernetes.io/blog/2026/01/29/ingress-nginx-statement/). After retirement there are no more releases for bug fixes, security patches, or updates of any kind, so any newly discovered vulnerability will remain unpatched. See the [Kubernetes retirement announcement](https://www.kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/).
>
> Nuclio builds its ingresses with `nginx.ingress.kubernetes.io/*` annotations (authentication, rewrites, canary, timeouts, and more), so it currently supports only controllers that understand the nginx annotation dialect. Not every ingress controller will work. The supported option is [Traefik](https://doc.traefik.io/traefik/reference/install-configuration/providers/kubernetes/kubernetes-ingress-nginx/) with its ingress-nginx compatibility provider (`kubernetesIngressNGINX`), as shown below.

In this guide, you'll set up a [Traefik](https://doc.traefik.io/traefik/) controller with the ingress-nginx compatibility provider enabled. Nuclio's default ingress class name is `nginx` (see `defaultHTTPIngressClassName` in the [platform configuration](../../tasks/configuring-a-platform.md)), so the cluster needs an `IngressClass` with that name, handled by the provider. The provider is available in Traefik Proxy v3.6.2 and later; see the [Traefik documentation](https://doc.traefik.io/traefik/reference/install-configuration/providers/kubernetes/kubernetes-ingress-nginx/) for its current status and the list of supported annotations.

Using [helm](https://helm.sh/) (provided `helm` is installed), run the following commands:
```sh
helm repo add traefik https://traefik.github.io/charts
helm repo update
helm upgrade --install traefik traefik/traefik \
  --namespace traefik --create-namespace \
  --set service.type=NodePort \
  --set providers.kubernetesIngressNGINX.enabled=true
```

The Traefik chart does not create the `IngressClass` for this provider, so create it yourself (if you never installed `ingress-nginx` on the cluster, it does not exist yet):
```sh
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: nginx
spec:
  controller: k8s.io/ingress-nginx
EOF
```

Verify that the controller is up by running the `kubectl --namespace=traefik get pods` command, and then run the `kubectl --namespace=traefik get service traefik` command to get the ingress NodePort. Following is a sample output for NodePort 30019:

```text
NAME      TYPE       CLUSTER-IP    EXTERNAL-IP   PORT(S)                      AGE
traefik   NodePort   10.96.12.34   <none>        80:30019/TCP,443:31234/TCP   1m
```

> **Note:** You must ensure that all your requests are sent to the returned NodePort.

Run the following command to deploy the sample `helloworld` function; (the command assumes the use of Minikube):
```sh
nuctl deploy -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go --registry $(minikube ip):5000 helloworld --run-registry localhost:5000
```

And now, invoke the function by its path.
Replace `<NodePort>` with the NodePort of your ingress controller, and replace `${minikube ip)` with your cluster IP if you are not using Minikube:
```sh
curl $(minikube ip):<NodePort>/helloworld/latest
```

For example, for NodePort 30019, run this command:
```sh
curl $(minikube ip):30019/helloworld/latest
```

## Customizing function ingress

By default, functions initialize the HTTP trigger and register `<function name>/latest`. However, you might want to add paths for functions to organize them in namespaces/groups, or even choose through which domain your functions can be triggered. To do this, you can configure your HTTP trigger in the [function's configuration](../../reference/function-configuration/function-configuration-reference.md). For example:

```yaml
  ...
  triggers:
    http:
      numWorkers: 4
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

If your `helloworld` function was configured in this way, and assuming that Traefik's NodePort is 30019, the function would be accessible through any of the following URLs:

- `<cluster ip>:30019/helloworld/latest`
- `some.host.com:30019/helloworld/latest`
- `some.host.com:30019/first/path`
- `some.host.com:30019/second/path`
- `<cluster ip>:30019/wat`
- `some.host.com:30019/wat`

Note that since the `i1` configuration explicitly specifies `some.host.com` as the `host` for the paths, the function will _not_ be accessible through the cluster IP; i.e., `<cluster ip>:30019/first/path` will return a `404` error.

## Authentication and ingress

> **Feature flag:** authentication requires `authentication.functionAuthenticationEnabled: true` in the platform
> configuration (disabled by default). Also set `authentication.authURL` and `authentication.signInURL` — these replace
> the legacy `ingressConfig.iguazioAuthURL` / `ingressConfig.iguazioSignInURL` fields.

All traffic routed through ingress is subject to the same authentication enforced by the auth-proxy sidecar inside the function pod. Configure the `authenticationMode` attribute in the HTTP trigger to control how the sidecar handles unauthenticated requests (see [HTTP trigger reference](../../reference/triggers/http.md)):

- `none` (default) — all requests are allowed.
- `api` — unauthenticated requests are rejected with HTTP 401.
- `browser` — unauthenticated requests are redirected (HTTP 302) to the configured sign-in URL.
- `basicAuth` — requests must carry valid HTTP Basic credentials.

The `/__internal/health` path is always allowed regardless of `authenticationMode`, so kubelet health probes work even when authentication is enabled.

When the auth-proxy operates in **auth-only** mode (DLX / scale-to-zero), the `X-Nuclio-Target-Function-Name` header identifies which function the request targets.

## Deploying an ingress example

Let's put this into practice and deploy the [ingress example](https://github.com/nuclio/nuclio/tree/development/hack/examples/golang/ingress/ingress.go). This is the **function.yaml** file for the example:

```yaml
apiVersion: "nuclio.io/v1"
kind: "NuclioFunction"
spec:
  runtime: "golang"
  triggers:
    http:
      numWorkers: 8
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

### Deploy the function

Deploy the function with the `nuctl` CLI. If you did not use Minikube, replace `$(minikube ip):5000` in the following command with your cluster IP:
```sh
nuctl deploy -p https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/ingress/ingress.go --registry $(minikube ip):5000 ingress --run-registry localhost:5000 --verbose
```

Behind the scenes, `nuctl` populates a function CR, which is picked up by the Nuclio `controller`. The `controller` iterates through all the triggers and looks for the required ingresses. For each ingress, the controller creates a Kubernetes Ingress object, which triggers the Traefik ingress controller to reconfigure the reverse proxy. Following are sample `controller` logs:

```
controller.functiondep (D) Adding ingress {"function": "helloworld", "host": "", "paths": ["/helloworld/latest"]}
controller.functiondep (D) Adding ingress {"function": "helloworld", "host": "my.host.com", "paths": ["/first/from/host"]}
controller.functiondep (D) Adding ingress {"function": "helloworld", "host": "", "paths": ["/first/path", "/second/path"]
```

### Invoke the function with nuctl

Invoke the function with `nuctl`, which will use the configured NodePort:
```sh
nuctl invoke ingress
```
Following is a sample output for this command:
```text
> Response headers:
Server = nuclio
Date = Thu, 02 Nov 2017 02:11:32 GMT
Content-Type = text/plain; charset=utf-8
Content-Length = 14

> Response body:
Handler called
```

### Configure a custom host

Add `my.host.com` to your local **/etc/hosts** file so that it resolves to your cluster IP. The following command assumes the use of Minikube:
```sh
echo "$(minikube ip) my.host.com" | sudo tee -a /etc/hosts
```

### Invoke the function with curl

Now, do some invocations with curl. The following examples assume the use of Minikube (except were your configured host is used) and NodePort 30019.

> **Note:** The parenthesized "works" and error indications at the end of each line signify the expected outcome and are not part of the command.

```sh
curl $(minikube ip):30019/ingress/latest (works)
curl my.host.com:30019/ingress/latest (works)

curl $(minikube ip):30019/first/path (works)
curl my.host.com:30019/first/path (works)

curl my.host.com:30019/first/from/host (works)
curl $(minikube ip):30019/first/from/host (404 error)
```

