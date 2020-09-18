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

Nuclio functions may at times need to return file contents in the body of the response. This has always been possible by simply reading the file contents into memory and setting that as the response body. For large files, however, this posed a problem - function memory consumption would often be x3-x5 the size of the file per worker. This is due to how Go manages its memory (growing the heap exponentially), how the fasthttp server retains response buffers and the fact that allocation may also happen in a secondary runtime like Python.

To alleviate this, a simple mechanism is provided to instruct the HTTP trigger itself to stream the contents of a file back as a response. This means that the function handler no longer needs to load anything into memory and the client should receive the first byte of the response much quicker.

The HTTP trigger normally uses [fasthttp](https://github.com/valyala/fasthttp) for its HTTP server implementation over Go's standard net/http server. fasthttp is a fantastic high performance HTTP server, proven in production and allowing for a very high amount of requests per second. Unfortunately, it does not support streaming back responses and mandates holding the entire response in memory. To support file streaming, we need to leverage the standard net/http server at the expense of performance (mostly requests per second). As such, any function that needs to stream back files MUST specify that the HTTP trigger should support streaming. Functions that do not require streaming will not experience reduced performance.

> Note: in 1.3.x and 1.4.x, the HTTP trigger can be configured to support streaming by adding an annotation of "nuclio.io/http-kind" set to "nethttp". In 1.5.x onwards, this will be part of the HTTP trigger configuration in the UI

Once the HTTP trigger is configured, should a function handler wish to stream back a file - it simply needs to set the following headers in its response (case insensitive):

- **X-nuclio-filestream-path**: The path of the file to be streamed back as a response. This must point to a file that is readable by the function container (e.g. in a volume or the local filesystem)
- **X-nuclio-filestream-delete-after-send**: Delete the local file after sending it back. This can be set to any string value

These headers will not be sent back to the client, but any other headers attached to the response will be returned normally. When providing these headers, the body of the response is ignored and should be empty. Any and all other HTTP trigger functionality continues to be supported when enabling file streaming.

Below are a few examples of handlers returning specifying file streaming:

Golang:
```golang
func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	return nuclio.Response{
		StatusCode:  200,
		ContentType: "text/x-yaml",
		Headers: map[string]interface{}{
			"some-additional-header": "some-value",
			"X-nuclio-filestream-path": "/etc/nuclio/config/processor/processor.yaml",
		},
	}, nil
}
```

Python:
```python
def handler(context, event):
    return context.Response(
        status_code=200,
        content_type='text/x-yaml',
        headers={
            'some-additional-header': 'some-value',
            'x-nuclio-filestream-path': '/etc/nuclio/config/processor/processor.yaml'
        }
    )
```

NodeJS:
```javascript
exports.handler = function(context, event) {
    context.callback(new context.Response("some body", 
                                          {
                                          	"some-additional-header": "some-value",
                                            "X-nuclio-filestream-path": "/etc/nuclio/config/processor/processor.yaml"  
                                          },
                                          "text/x-yaml",
                                          200));
};
```

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

