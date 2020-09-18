# http: HTTP Trigger

The HTTP trigger is the only trigger created by default if not configured (by default, it has 1 worker). This trigger handles incoming HTTP requests at container port 8080, assigning workers to incoming requests. If a worker is not available, a `503` error is returned.

## Attributes

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| port | int | The NodePort (or equivalent) on which the function will serve HTTP requests. If empty, chooses a random port within the platform range. |
| ingresses.(name).host | string | The host to which the ingress maps. |
| ingresses.(name).paths | list of strings | The paths that the ingress handles. Variables of the form `{{.<NAME>}}` can be specified using `.Name`, `.Namespace`, and `.Version`. For example, `/{{.Namespace}}-{{.Name}}/{{.Version}}` will result in a default ingress of `/namespace-name/version`. |

## File streaming (Experimental)

> Note: This feature is highly experimental and limited to 1.3.28+. It will be ported to 1.4.x and 1.5.x in the future

Nuclio functions may at times need to return file contents in the body of the response. This has always been possible by simply reading the file contents into memory and setting the response body to the contents. For large files, however, this posed a problem - function memory consumption would often be x3-x5 the size of the file per worker.  

### Examples

Without ingresseses -

```yaml
triggers:
  myHttpTrigger:
    maxWorkers: 4
    kind: "http"
    attributes:
      port: 32001
```

With ingresseses -

```yaml
triggers:
  myHttpTrigger:
    maxWorkers: 4
    kind: "http"
    attributes:
      port: 32001
  
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

