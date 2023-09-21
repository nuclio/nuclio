# Deploying Functions

This tutorial guides you through the process of deploying functions and specifying the function configuration.

#### In this document
- [Deploy a function with the Nuclio dashboard](#deploy-a-function-with-the-nuclio-dashboard)
- [Writing a simple function (nuctl)](#writing-a-simple-function)
- [Deploying a simple function (nuctl)](#deploying-a-simple-function)
- [Providing function configuration (nuctl)](#providing-function-configuration)
- [Exposing a function](#exposing-a-function)
- [What's next](#whats-next)


<a id="deploy-a-function-with-the-nuclio-dashboard"></a>
## Deploy a function with the Nuclio dashboard

Browse to `http://localhost:8070` (after having forwarded this port as part of the Nuclio installation) to see the Nuclio Dashboard.

> NOTE: if running on kubernetes, you may want to port-forward nuclio dashboard to your localhost using below command:

```shell
kubectl port-forward -n nuclio $(kubectl get pods -n nuclio -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') 8070:8070
```

Select the "default" project and then select **New Function** from the action toolbar to display the **Create function** page (http://localhost:8070/projects/default/create-function).  
Choose one of the predefined template functions, and select **Deploy**.  
The first build populates the local Docker cache with base images and other files, so it might take a while to complete, depending on your network.  
When the function deployment completes, you can select **Test** to invoke the function with a body.

<a id="writing-a-simple-function"></a>
## Writing a simple function

After successfully installing Nuclio, you can start writing functions and deploying them to your cluster. All supported runtimes (such as Go, Python, or NodeJS) have an entry point that receives two arguments:

- **Context:** An object that maintains state across function invocations. Includes objects like the logger, data bindings, worker information and user specified data. See the appropriate context reference for your specific runtime for more
- **Event:** An object containing information about the event that triggered the function including body, headers, trigger information and so forth

The entry point, essentially a function native to the runtime, is called whenever one of the configured triggers receives an event (more on configuring triggers later).

> **Note:** Nuclio supports configuring multiple triggers for a single function. For example, the same function can be called both via calling an HTTP endpoint and posting to a Kafka stream. Some functions can behave uniformly, as accessing many properties of the event is identical regardless of triggers (for example, `event.GetBody()`). Others may want to behave differently, using the event's trigger information to determine through which trigger it arrived.

The entry point may return a response which is handled differently based on which trigger configured the function. Some synchronous triggers (like HTTP) expect a response, some (like RabbitMQ) expect an ack or nack and others (like cron) ignore the response altogether.

To put this in Python code, an entry point is a simple function with two arguments and a return value:

```python
import os


def my_entry_point(context, event):

	# use the logger, outputting the event body
	context.logger.info_with('Got invoked',
		trigger_kind=event.trigger.kind,
		event_body=event.body,
		some_env=os.environ.get('MY_ENV_VALUE'))

	# check if the event came from cron
	if event.trigger.kind == 'cron':

		# log something
		context.logger.info('Invoked from cron')

	else:

		# return a response
		return 'A string response'
```
<a id="deploying-a-simple-function"></a>
## Deploying a simple function

To convert source code to a running function, you must first _deploy_ the function. A deployment process has three stages:

1. The source code is built to a container image and pushed to a Docker registry
2. A function object is created in Nuclio (i.e., in Kubernetes, this is a function CRD)
3. A controller creates the appropriate function resources on the cluster (i.e., in Kubernetes this is the deployment, service, ingress, etc.)

This process can be triggered through `nuctl deploy` which you will use throughout this tutorial. You will now write
the function that you wrote in the previous step to a `/tmp/nuclio/my_function.py` file. Before you do anything,
verify with `nuctl` that everything is properly configured by getting all functions deployed in the "nuclio" namespace:

```sh
nuctl get function --namespace nuclio

No functions found
```

Now deploy your function, specifying the function name, the path, the "nuclio" namespace to which all setup guides expect functions to go to and applicable registry information:

```sh
nuctl deploy my-function \
	--namespace nuclio \
	--path /tmp/nuclio/my_function.py \
	--runtime python \
	--handler my_function:my_entry_point \
	--http-trigger-service-type nodePort \
	--registry <registry-url> \
	--run-registry <registry-url>
```

> **Notes:**
1. `--path` can also hold a URL.
2. See the applicable setup guide to get registry information.
3. Notice we used a `nodePort` to expose the function and make it reachable externally. This
> is for demonstration purposes only. See [exposing a function](#exposing-a-function) to learn more about why this is here.
4. Replace <registry-url> with your docker registry (e.g.: `$(minikube ip):5000` for minikube or `<registry-name>.azurecr.io` for AKS)

Once the function deploys, you should see `Function deploy complete` and an HTTP port through which you can invoke it. If there's a problem, invoke the above with `--verbose` and try to understand what went wrong. You can see your function through `nuctl get`:

```sh
nuctl get function --namespace nuclio

  NAMESPACE |    NAME     | VERSION | STATE | NODE PORT | REPLICAS
  nuclio    | my-function | latest  | ready |     ?     | 1/1

```

To illustrate that the function is indeed accessible via HTTP, you'll use [httpie](https://httpie.org) to invoke
the function at the port specified by the deployment log:

```sh
http <external-ip-address>:<port from log>

HTTP/1.1 200 OK
Content-Length: 17
Content-Type: text/plain
Date: Mon, 05 Mar 2018 09:36:05 GMT
Server: nuclio

A string response
```

> NOTE: if running in minikube, replace external-ip-address with `$(minikube ip)`

You can use `nuctl invoke` to invoke the function by name, and even get function logs in the process:

```sh
nuctl invoke my-function --namespace nuclio --via external-ip

    nuctl.platform.invoker (I) Executing function {"method": "GET", "url": "http://192.168.64.8:30521", "body": {}}
    nuctl.platform.invoker (I) Got response {"status": "200 OK"}
                     nuctl (I) >>> Start of function logs
                     nuctl (I) Got invoked {"trigger_kind": "http", "some_env": null, "event_body": "", "time": 1520245355728.884}
                     nuctl (I) <<< End of function logs

> Response headers:
Server = nuclio
Date = Mon, 05 Mar 2018 10:22:35 GMT
Content-Type = text/plain
Content-Length = 17

> Response body:
A string response
```

<a id="providing-function-configuration"></a>
## Providing function configuration

There are often cases in which providing code is not enough to deploy a function. For example, if

- The function expects environment variables or secrets
- You would like to trigger the function through Kafka, Kinesis, etc. These require configuration to connect to the data source
- There are third-party dependencies or additional files (both language packages and OS) that need to reside alongside the function

For such cases and many others you need to provide a function configuration alongside your function code. Nuclio provides you with several mechanisms for providing the function configuration:

- A **function.yaml** file
- Inline configuration by means of crafting a comment in your code that contains the **function.yaml** contents
- Command-line arguments for the Nuclio CLI (`nuctl`). This argument will override the **function.yaml** configuration, if present
- The UI, through the **Configuration** tab

While there are several mechanisms to provide the configuration, there is only one configuration schema. In the following examples, you'll set an environment variable (`MY_ENV_VALUE`) and add a cron trigger through `nuctl`, a `function.yaml` file and inline configuration.

After you provide this configuration, you can invoke the function and notes that `MY_ENV_VALUE` is now set to `my value`:

```sh
nuctl invoke my-function --namespace nuclio --via external-ip

    nuctl.platform.invoker (I) Executing function {"method": "GET", "url": "http://192.168.64.8:30521", "body": {}}
    nuctl.platform.invoker (I) Got response {"status": "200 OK"}
                     nuctl (I) >>> Start of function logs
                     nuctl (I) Got invoked {"some_env": "my value", "event_body": "", "time": 1520246616537.9287, "trigger_kind": "http"}
                     nuctl (I) <<< End of function logs

> Response headers:
Server = nuclio
Date = Mon, 05 Mar 2018 10:43:35 GMT
Content-Type = text/plain
Content-Length = 17

> Response body:
A string response
```

If you were to look at the function logs through `kubectl` (assuming you're deploying to Kubernetes), you'd see the function being invoked periodically, where `Invoked from cron` is logged as well:

```sh
...
    processor.cron (I) Got invoked {"trigger_kind": "cron", "some_env": "my value", "event_body": ""}
    processor.cron (I) Invoked from cron
    processor.cron (I) Got invoked {"trigger_kind": "cron", "some_env": "my value", "event_body": ""}
    processor.cron (I) Invoked from cron
...
```

### Providing configuration via nuctl

With `nuctl`, you simply pass `--env` and a JSON encoding of the trigger configuration:

```sh
nuctl deploy my-function \
    --namespace nuclio \
    --path /tmp/nuclio/my_function.py \
    --runtime python \
    --handler my_function:my_entry_point \
    --http-trigger-service-type nodePort \
    --registry $(minikube ip):5000 \
    --run-registry localhost:5000 \
    --env MY_ENV_VALUE='my value' \
    --triggers '{"periodic": {"kind": "cron", "attributes": {"interval": "3s"}}}'
```

### Providing configuration via function.yaml

For a more manageable approach, you can keep your configuration alongside your source in the same directory. Create a `/tmp/nuclio/function.yaml` file with the following contents:

```yaml
apiVersion: "nuclio.io/v1"
kind: NuclioFunction
metadata:
  name: my-function
  namespace: nuclio
spec:
  env:
  - name: MY_ENV_VALUE
    value: my value
  handler: my_function:my_entry_point
  runtime: python
  triggers:
    http:
      kind: http
      attributes:
        serviceType: NodePort
    periodic:
      attributes:
        interval: 3s
      class: ""
      kind: cron
```

With all the information in the `function.yaml`, you can pass the _directory_ of the source and configuration to `nuctl`. 
The name, namespace, trigger, env are all taken from the configuration file:

```sh
nuctl deploy \
    --path /tmp/nuclio \
    --registry $(minikube ip):5000 \
    --run-registry localhost:5000
```

### Providing configuration via inline configuration
Sometimes it's convenient to have the source and configuration bundled together in a single, human readable file. While it's not recommended for production, it's great for trying things out. To do this, you craft a special comment somewhere in your function source and provide the containing file as `path` (this will not work if `path` is a directory).

Write the following to `/tmp/nuclio/my_function_with_config.py`:

```python
import os

# @nuclio.configure
#
# function.yaml:
#   apiVersion: "nuclio.io/v1"
#   kind: NuclioFunction
#   metadata:
#     name: my-function
#     namespace: nuclio
#   spec:
#     env:
#     - name: MY_ENV_VALUE
#       value: my value
#     handler: my_function_with_config:my_entry_point
#     runtime: python
#     triggers:
#       http:
#         kind: http
#         attributes:
#           serviceType: NodePort
#       periodic:
#         attributes:
#           interval: 3s
#         class: ""
#         kind: cron

def my_entry_point(context, event):

	# use the logger, outputting the event body
	context.logger.info_with('Got invoked',
		trigger_kind=event.trigger.kind,
		event_body=event.body,
		some_env=os.environ.get('MY_ENV_VALUE'))

	# check if the event came from cron
	if event.trigger.kind == 'cron':

		# log something
		context.logger.info('Invoked from cron')

	else:

		# return a response
		return 'A string response'
```

Now deploy this function:

```sh
nuctl deploy \
    --path /tmp/nuclio/my_function_with_config.py \
    --registry $(minikube ip):5000 \
    --run-registry localhost:5000
```

<a id="exposing-a-function"></a>
## Exposing a function

>**Security note:** Exposing your functions outside your Kubernetes cluster network has dire security implications.
> Please make sure you understand the risks involved before deciding to expose any function externally. Always control
> on which networks your functions are exposed, and use proper authentication to protect them, as well as the rest of your pipeline and data.

When deploying a function on a Kubernetes cluster, the function is not exposed by default for external communication,
but only on the kubernetes cluster network, using a `ClusterIP` [Service Type](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types).
In most network topologies this makes the function only available inside the cluster network, and [Kubernetes network policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
can be used to further limit and control communications. The function will be unavailable for invocations
over HTTP from entities running outside the cluster, like `nuctl`, `curl` or any other HTTP client.

To understand whether a function is reachable to your HTTP client (this can be `curl`, `httpie`, `nuctl invoke` or any
other HTTP client), consider where it is running from and the network path. An unexposed (default) function will not be
reachable from your client unless you're running the client from inside a pod in your cluster.

If you wish to expose your function externally, for example, to be able to run `nuctl invoke` from outside the
Kubernetes network, you can do so in one of 2 ways during deployment, both controlled via the [HTTP trigger spec](/docs/reference/triggers/http.md):
1. Configure the function with a reachable [HTTP ingress](/docs/reference/triggers/http.md#attributes-ingresses). For
   this to work you'll need to install an ingress controller on your cluster. See [function ingress document](/docs/concepts/k8s/function-ingress.md)
   for more details.
2. Configure the function to use [serviceType](/docs/reference/triggers/http.md#attributes-serviceType) of type `nodePort`.

If you are deploying the function using [nuctl](/docs/reference/nuctl/nuctl.md) CLI, you can also configure a `nodePort` easily by using the
`--http-trigger-service-type=nodePort` CLI arg.

<a id="whats-next"></a>
## What's next?

- Check out how to [build functions once and deploy them many times](/docs/tasks/deploying-pre-built-functions.md).
- Read more about [function configuration](/docs/reference/function-configuration/function-configuration-reference.md).

