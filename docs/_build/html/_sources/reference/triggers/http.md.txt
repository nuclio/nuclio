# http: HTTP Trigger

**In This Document**

- [Overview](#overview)
- [Attributes](#attributes)
- [Examples](#examples)

<a id="overview"></a>
## Overview

The HTTP trigger is the only trigger created by default if not configured (by default, it has 1 worker). This trigger
handles incoming HTTP requests at container port 8080, assigning workers to incoming requests. If a worker is not
available, a `503` error is returned.

<a id="attributes"></a>
## Attributes

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| port | int | The NodePort (or equivalent) on which the function will serve HTTP requests. If empty, chooses a random port within the platform range. When running on k8s, this only has effect if [serviceType](/docs/reference/triggers/http.md#attributes-serviceType) of type `nodePort` is used|
| <a id="attributes-ingresses"></a>ingresses.(name).host | string | The host to which the ingress maps. |
| ingresses.(name).hostTemplate | string | The template used to generate an ingress host (use `@nuclio.fromDefault` for default template) |
| ingresses.(name).paths | list of strings | The paths that the ingress handles. Variables of the form `{{.<NAME>}}` can be specified using `.Name`, `.Namespace`, and `.Version`. For example, `/{{.Namespace}}-{{.Name}}/{{.Version}}` will result in a default ingress of `/namespace-name/version`. |
| readBufferSize | int | Per-connection buffer size for reading requests. |
| maxRequestBodySize | int | Maximum request body size. |
| reduceMemoryUsage | bool | Reduces memory usage at the cost of higher CPU usage if set to true. |
| cors.enabled | bool | `true` to enable cross-origin resource sharing (CORS); (default: `false`). |
| cors.allowOrigins | list of strings | Indicates that the CORS response can be shared with requesting code from the specified origin (`Access-Control-Allow-Origin` response header); (default: `['*']` to allow sharing with any origin, for requests without credentials). |
| cors.allowMethods | list of strings | The allowed HTTP methods, which can be used when accessing the resource (`Access-Control-Allow-Methods` response header); (default: `"HEAD, GET, POST, PUT, DELETE, OPTIONS"`). |
| cors.allowHeaders | list of strings | The allowed HTTP headers, which can be used when accessing the resource (`Access-Control-Allow-Headers` response header); (default: `"Accept, Content-Length, Content-Type, X-nuclio-log-level"`). |
| cors.allowCredentials | bool | `true` to allow user credentials in the actual request (`Access-Control-Allow-Credentials` response header); (default: `false`). |
| cors.preflightMaxAgeSeconds | int | The number of seconds in which the results of a preflight request can be cached in a preflight result cache (`Access-Control-Max-Age` response header); (default: `-1` to indicate no preflight results caching). |
| <a id="attributes-serviceType"></a>serviceType | string | (Kubernetes only) Kubernetes `ServiceType`, used by the Kubernetes service to expose the trigger. The default `ServiceType` is `ClusterIP`, which means that by default the trigger won't be exposed outside of the cluster unless you configure a proper ingress or manually change the `ServiceType` to `NodePort`. |

<a id="examples"></a>
## Examples

With 4 workers and a maximum body size of 1 KB -

```yaml
triggers:
  myHttpTrigger:
    maxWorkers: 4
    kind: "http"
    attributes:
      maxRequestBodySize: 1024
```

With a predefined port number -

```yaml
triggers:
  myHttpTrigger:
    kind: "http"
    attributes:
      port: 32001
```

With ingresses -

```yaml
triggers:
  myHttpTrigger:
    kind: "http"
    attributes:

      # See "Invoking Functions By Name With Kubernetes Ingresses" for more details
      # on configuring ingresses
      ingresses:
        templated-host:

          # e.g.: "my-func.some-namespace.nuclioio.com
          hostTemplate: "{{ .ResourceName }}.{{ Namespace }}.nuclioio.com"
          paths:
            - "/"

        http:
          host: "host.nuclio"
          paths:
            - "/first/path"
            - "/second/path"
        http2:
          paths:
            - "MyFunctions/{{.Name}}/{{.Version}}"
```

With CORS -

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
