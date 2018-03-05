# Deploying functions

This guide goes through deploying functions and how to specify function configuration.

#### In this document
- [Writing a simple function](#writing-a-simple-function)
- [Deploying a simple function](#deploying-a-simple-function)
- [Providing function configuration](#providing-function-configuration)

## Writing a simple function

After successfully installing nuclio, we can start writing functions and deploying them to our cluster. Regardless of the runtime we choose (e.g. Go, Python, NodeJS) have an entrypoint that receives two arguments:
- Context: An object that maintains state across function invocations. Includes objects like the logger, databindings, worker information and user specified data. See the appropriate context reference for your specific runtime for more
- Event: An object containing information about the event that triggered the function including body, headers, trigger information and so forth

The entrypoint, essentially a function native to the runtime, is called whenever one of the configured triggers receives an event (more on configuring triggers later).

> **Note:** nuclio supports configuring multiple triggers for a single function. For example, the same function can be called both via calling an HTTP endpoint and posting to a Kafka stream. Some functions can behave uniformly, as accessing many properties of the event is identical regardless of triggers (e.g. `event.GetBody()`) Others may want to behave differently, using the event's trigger information to determine through which trigger it arrived

The entrypoint may return a response which is handled differently based on which trigger configured the function. Some synchronous triggers (like HTTP) expect a response, some (like RabbitMQ) expect an ack or nack and others (like cron) ignore the response altogether.

To put this in Python code, an entrypoint is a simple function with two arguments and a return value:

```python
import os


def my_entrypoint(context, event):

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

## Deploying a simple function

To convert source code to a running function, we must first _deploy_ the function. A deploy process has three stages:
1. The source code is built to a docker image and pushed to a docker registry
2. A function object is created in nuclio (i.e. in Kubernetes, this is a function CRD)
3. A controller creates the appropriate function resources on the cluster (i.e. in Kubernetes this is the deployment, service, ingress, etc)

This process can be triggered through `nuctl deploy` which we will use throughout this guide. Let's go ahead and write the function above to `/tmp/nuclio/my_function.py`. Before we do anything, verify with `nuctl` that everything is properly configured by getting all functions deployed in the `nuclio` namespace:

```sh
nuctl get function --namespace nuclio

No functions found
```

Now deploy our function, specifying the function name, the path, the nuclio namespace to which all setup guides expect functions to go to and applicable registry information:

```sh
nuctl deploy my-function \
	--path /tmp/nuclio/my_function.py \
	--runtime python:2.7 \
	--handler my_function:my_entrypoint \
	--namespace nuclio \
	--registry $(minikube ip):5000 --run-registry localhost:5000
```

> **Note:**
> 1. `--path` can also hold a URL
> 2. See the applicable setup guide to get registry informatiom

Once the function deploys, you should see `Function deploy complete` and an HTTP port through which we can invoke it. If there's a problem, invoke the above with `--verbose` and try to understand what went wrong. We can see our function through `nuctl get`:

```sh
nuctl get function --namespace nuclio

  NAMESPACE |    NAME     | VERSION | STATE | NODE PORT | REPLICAS
  nuclio    | my-function | latest  | ready |     ?     | 1/1

```

To illustrate that the function is indeed accessible via HTTP, we'll use [httpie](https://httpie.org) to invoke the function at the port specified by the deploy log:

```sh
http $(minikube ip):<port from log>

HTTP/1.1 200 OK
Content-Length: 17
Content-Type: text/plain
Date: Mon, 05 Mar 2018 09:36:05 GMT
Server: nuclio

A string response
```

We can use `nuctl invoke` to invoke the function by name, and even get function logs in the process:

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

## Providing function configuration

There are often cases in which providing code is not enough to deploy a function. For example, if

- The function expects environment variables or secrets
- You would like to trigger the function through Kafka, Kinesis, etc. These require configuration to connect to the data source
- There are third-party dependencies or additional files (both language packages and OS) that need to reside alongside the function

For such cases and many others you need to provide a function configuration alongside your function code. nuclio provides you with several mechanisms for providing the function configuration:

- A **function.yaml** file
- Inline configuration by means of crafting a comment in your code that contains the **function.yaml** contents
- Command-line arguments for the nuclio CLI (`nuctl`). This argument will override the **function.yaml** configuration, if present
- The UI, through the **Configuration** tab

While there are several mechanisms to provide the configuration, there is only one configuration schema. In the following examples, we'll set an environment variable (`MY_ENV_VALUE`) and add a cron trigger through nuctl, a `function.yaml` file and inline configuration.

After we provide this configuration, we can invoke the function and notes that `MY_ENV_VALUE` is now set to `my value`:

```
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

If we were to look at the function logs through `kubectl` (assuming we're deploying to Kubernetes), we'd see the function being invoked periodically, where `Invoked from cron` is logged as well:

```
            processor.cron (I) Got invoked {"trigger_kind": "cron", "some_env": "my value", "event_body": ""}
            processor.cron (I) Invoked from cron
 sor.cron.w0.python.logger (D) Event executed {"name": "", "status": 200, "eventID": "4d9dde6d-a841-4214-adb4-4118d3b5f32b"}
            processor.cron (D) Waiting for next event {"delay": "2.997526523s"}
            processor.cron (D) Submitting event
 sor.cron.w0.python.logger (D) Processing event {"name": "", "version": -1, "eventID": "3225a663-1575-47f7-b28d-8dfa093a64d8"}
 sor.cron.w0.python.logger (D) Sending event to wrapper {"size": 0}
l{"message": "Got invoked", "level": "info", "with": {"trigger_kind": "cron", "some_env": "my value", "event_body": ""}, "datetime": "2018-03-05 10:46:00,000"}
l{"message": "Invoked from cron", "level": "info", "with": {}, "datetime": "2018-03-05 10:46:00,001"}
            processor.cron (I) Got invoked {"trigger_kind": "cron", "some_env": "my value", "event_body": ""}
            processor.cron (I) Invoked from cron
 sor.cron.w0.python.logger (D) Event executed {"name": "", "status": 200, "eventID": "3225a663-1575-47f7-b28d-8dfa093a64d8"}
            processor.cron (D) Waiting for next event {"delay": "2.998278626s"}
            processor.cron (D) Submitting event
 sor.cron.w0.python.logger (D) Processing event {"name": "", "version": -1, "eventID": "2437d117-1871-41d1-8e10-4eafe2d34523"}
 sor.cron.w0.python.logger (D) Sending event to wrapper {"size": 0}
l{"message": "Got invoked", "level": "info", "with": {"trigger_kind": "cron", "some_env": "my value", "event_body": ""}, "datetime": "2018-03-05 10:46:03,000"}
l{"message": "Invoked from cron", "level": "info", "with": {}, "datetime": "2018-03-05 10:46:03,001"}
1
```

### Providing configuration via nuctl

With `nuctl`, we simply pass `--env` and a JSON encoding of the trigger configuration:

```
nuctl deploy my-function \
	--path /tmp/nuclio/my_function.py \
	--runtime python:2.7 \
	--handler my_function:my_entrypoint \
	--namespace nuclio \
	--registry $(minikube ip):5000 --run-registry localhost:5000 \
	--env MY_ENV_VALUE='my value' \
	--triggers '{"periodic": {"kind": "cron", "attributes": {"interval": "3s"}}}'
```

### Providing configuration via function.yaml

For a more manageable approach, we can keep our configuration alongside our source in the same directory. Create a `/tmp/nuclio/function.yaml` file with the following contents:

```yaml
metadata:
  name: my-function
  namespace: nuclio
spec:
  env:
  - name: MY_ENV_VALUE
    value: my value
  handler: my_function:my_entrypoint
  runtime: python:2.7
  triggers:
    periodic:
      attributes:
        interval: 3s
      class: ""
      kind: cron
```

With all the information in the `function.yaml`, we can pass the _directory_ of the source and configuration to `nuctl`. The name, namespace, trigger, env are all taken from the configuration file:

```
nuctl deploy --path /tmp/nuclio \
	--registry $(minikube ip):5000 --run-registry localhost:5000
```

### Providing configuration via inline configuration
Sometimes it's convenient to have the source and configuration bundled together in a single, human readable file. While it's not recommended for production, it's great for trying things out. To do this, we craft a special comment somewhere in our function source and provide the containing file as `path` (this will not work if `path` is a directory).

Write the following to `/tmp/nuclio/my_function_with_config.py`:

```
import os

# @nuclio.configure
#
# function.yaml:
#   metadata:
#     name: my-function
#     namespace: nuclio
#   spec:
#     env:
#     - name: MY_ENV_VALUE
#       value: my value
#     handler: my_function:my_entrypoint
#     runtime: python:2.7
#     triggers:
#       periodic:
#         attributes:
#           interval: 3s
#         class: ""
#         kind: cron

def my_entrypoint(context, event):

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
