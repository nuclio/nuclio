# http: HTTP Trigger

The HTTP trigger is the only trigger created by default if not configured (by default, it has 1 worker). This trigger handles incoming HTTP requests at container port 8080, assigning workers to incoming requests. If a worker is not available, a `503` error is returned.

## Attributes

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| port | int | The NodePort (or equivalent) on which the function will serve HTTP requests. If empty, chooses a random port within the platform range. |
| ingresses.(name).host | string | The host to which the ingress maps. |
| ingresses.(name).paths | list of strings | The paths that the ingress handles. Variables of the form `{{.<NAME>}}` can be specified using `.Name`, `.Namespace`, and `.Version`. For example, `/{{.Namespace}}-{{.Name}}/{{.Version}}` will result in a default ingress of `/namespace-name/version`. |
| readBufferSize | int | Per-connection buffer size for requests' reading. |
| cors.enabled | boolean | Whether cors is enabled per function. |
| cors.allowOrigin | string | Indicates where the CORS originates from (Default: '*'). |
| cors.allowMethods | list of strings | Indicates which methods can be used during the actual request (Default: HEAD, GET, POST, PUT, DELETE, OPTIONS). |
| cors.allowHeaders | list of strings | Indicates which header field names can be used during the actual request (Default: Accept, Content-Length, Content-Type, X-nuclio-log-level). |
| cors.allowCredentials | boolean | Indicates that the actual request can include user credentials (Default: false). |
| cors.preflightMaxAgeSeconds | int | Indicates how long the results of a preflight request can be cached in a preflight result cache (Default: -1, not cached) |

### Examples

With 4 workers -

```yaml
triggers:
  myHttpTrigger:
    maxWorkers: 4
    kind: "http"
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
      CORS:
        enabled: true
        allowOrigin: "foo.bar"
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