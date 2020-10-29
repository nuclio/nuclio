# http: HTTP Trigger

The HTTP trigger is the only trigger created by default if not configured (by default, it has 1 worker). This trigger handles incoming HTTP requests at container port 8080, assigning workers to incoming requests. If a worker is not available, a `503` error is returned.

## Attributes

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| port | int | The NodePort (or equivalent) on which the function will serve HTTP requests. If empty, chooses a random port within the platform range. |
| ingresses.(name).host | string | The host to which the ingress maps. |
| ingresses.(name).paths | list of strings | The paths that the ingress handles. Variables of the form `{{.<NAME>}}` can be specified using `.Name`, `.Namespace`, and `.Version`. For example, `/{{.Namespace}}-{{.Name}}/{{.Version}}` will result in a default ingress of `/namespace-name/version`. |
| readBufferSize | int | Per-connection buffer size for reading requests. |
| maxRequestBodySize | int | Maximum request body size. |
| cors.enabled | bool | `true` to enable cross-origin resource sharing (CORS); (default: `false`). |
| cors.allowOrigins | list of strings | Indicates that the CORS response can be shared with requesting code from the specified origin (`Access-Control-Allow-Origin` response header); (default: `['*']` to allow sharing with any origin, for requests without credentials). |
| cors.allowMethods | list of strings | The allowed HTTP methods, which can be used when accessing the resource (`Access-Control-Allow-Methods` response header); (default: `"HEAD, GET, POST, PUT, DELETE, OPTIONS"`). |
| cors.allowHeaders | list of strings | The allowed HTTP headers, which can be used when accessing the resource (`Access-Control-Allow-Headers` response header); (default: `"Accept, Content-Length, Content-Type, X-nuclio-log-level"`). |
| cors.allowCredentials | bool | `true` to allow user credentials in the actual request (`Access-Control-Allow-Credentials` response header); (default: `false`). |
| cors.preflightMaxAgeSeconds | int | The number of seconds in which the results of a preflight request can be cached in a preflight result cache (`Access-Control-Max-Age` response header); (default: `-1` to indicate no preflight results caching). |
| serviceType | string | (k8s only) ServiceType for the k8s service created for exposing the trigger. **Important Note:** As of nuclio version 1.5.2 the default ServiceType is `ClusterIP` as opposed to `NodePort`. This means by default the function will not be exposed outside the cluster without configuring a proper ingress or manually changing the ServiceType to `NodePort` |

### Examples

With 4 workers and maximum body size of 1kb -

```yaml
triggers:
  myHttpTrigger:
    maxWorkers: 4
    kind: "http"
    attributes:
      maxRequestBodySize: 1024
```

With predefined port number -

```yaml
triggers:
  myHttpTrigger:
    kind: "http"
    attributes:
      port: 32001
```

With ingresseses -

```yaml
triggers:
  myHttpTrigger:
    kind: "http"
    attributes:
  
      # See "Invoking Functions By Name With Kubernetes Ingresses" for more details
      # on configuring ingresses
      ingresses:
        http:
          host: "host.nuclio"
          paths:
          - "/first/path"
          - "/second/path"
        http2:
          paths:
          - "MyFunctions/{{.Name}}/{{.Version}}"
```

with CORS -

```yaml
triggers:
  myCORSHttpTrigger:
    kind: "http"
    attributes:
      cors:
        enabled: true
        allowOrigins:
          - "foo.bar"
        allowHeaders:
          - "Accept"
          - "Content-Length"
          - "Content-Type"
          - "X-nuclio-log-level"
          - "MyCustomAllowedRequestHeader"
        allowMethods:
          - "GET"
          - "HEAD"
          - "POST"
          - "PATCH"
        allowCredentials: false
        preflightMaxAgeSeconds: 3600
```
