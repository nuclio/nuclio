# Function Configuration and Metadata
nuclio function configuration and metadata is described in the function spec, a user can create the spec as a YAML or JSON file, push it through the API, or use the CLI to define or override one.

Configurations can also be specified in the function code header using remarks and decorations (can see in the examples):

**Note:** the examples below will be using YAML, you can use JSON as well.

The basic structure resembles Kubernetes resource definitions, it includes the apiVersion, kind, metadata, spec, and status sections. The “status” section is returned by the controller (in a get operation) and should not be specified.
See a minimal definition below:

```yaml
apiVersion: "nuclio.io/v1"
kind: Function
metadata:
  name: example
spec:
  image: example:latest
```

## Metadata

The **metadata** section include the following attributes:
 - **name** - name of the function
 - **namespace** - the kubernetes namespace (can be viewed as an independent project)
 - **labels** - a list of key/value tags used for looking up the function, note that "function name", "version", and "alias" are reserved and filled automatically by the controller
 - **annotations** - list of key/value based annotations

## Requierment Spec

The **spec** secion contains the requierments and attributes and has the following elements:

 - **description** (string) - free description of the function
 - **handler** (string) - the name of the function handler call (nuclio will try to auto detect that)
 - **runtime** (string) - name of the language runtime (nuclio will try to auto detect that)
 - **code** - a structure containing the source code or its location of and access credentials
 - **image** (string) - full path to the function artifact (container image), note you can either specify the code or the already built image but not both.
 - **env** - a name/value environment variable tuple, it is also possible to point to secrets as described in the following example
 - **resources** - specify the requested and limit of CPU and Memory resource (similar to Kubernetes pod resources definition)
 - **replicas** (int) - number of desired instances, 0 for auto-scaling
 - **minReplicas** (int) - minimum number of replicas
 - **maxReplicas** (int) - maximum number of replicas
 - **disable** (boolean) - can be set to True to disable a function
 - **dataBindings** - describe a list of data resources used by the function (currently limited to iguazio platform)
 - **triggers** - a list of event sources and their configuration, see examples below. trigger name must be unique per function and all its versions (in future it will be possible to move triggers between versions or have the same trigger feed multiple function versions for canary deployments)

**Note:** other fields are not fully supported yet, and will be documented when they will be completed.

when creating a function using the CLI **run** command each one of the properties above can be specified or overritten using a command line argument, type `nuctl run --help` for details.

## Complete Example (YAML)

```yaml
apiVersion: "nuclio.io/v1"
kind: Function
metadata:
  name: example
  namespace: myproject
  labels:
    author: joe
spec:
  image: example:latest
  replicas: 0
  maxReplicas: 10

  env:
  - name: SOME_ENV
    value: abc
  - name: SECRET_PASSWORD_ENV_VAR
    valueFrom:
      secretKeyRef:
        name: my-secret
        key: password

  resources:
    requests:
      memory: "64Mi"
      cpu: "250m"
    limits:
      memory: "128Mi"
      cpu: "500m"

  triggers:
    # for HTTP triggers (API gateway) to work a Kubernetes ingress controller should be installed
    # see the getting started guide for more details
    http:
      maxWorkers: 4
      kind: "http"
      listenAddress: ":8080" # not really configurable right now, expect 8080
      attributes:
        ingresses:
          http:
            host: "host.nuclio"
            paths:
            - "/first/path"
            - "/second/path"
          http2:
            paths:
            - "/wat"

    rmqFunctions:
      kind: "rabbit-mq"
      url: "amqp://user:pass@10.0.0.1:5672"
      attributes:
        exchangeName: "functions"
        queueName: "functions"

    someKafkaTopic:
      kind: "kafka"
      url: "10.0.0.2"
      attributes:
        topic: "my.topic"
        partitions: [0, 5, 10]

    someKinesisStream:
      kind: "kinesis"
      attributes:
        accessKeyID: "my-key"
        secretAccessKey: "my-secret"
        regionName: "eu-west-1"
        streamName: "my-stream"
        shards: [0, 1, 2]

    someNatsTopic:
      kind: "nats"
      url: "10.0.0.3"
      attribtes:
        "topic": "my.topic"

  dataBindings:
    db0:
      class: "v3io"
      secret: "something"
      url: "http://192.168.51.240:8081/1024"
```

the example above demonstrates how we can use Kubernetes namespaces, specify labels, use environment variables and secrets, and specify exact memory and cpu resources. For the example to work the Kubernetes namespace `myproject` and the secret `my-secret` must be defined ahead of time.

**Note:** when specifying labels you can list functions based on a specific label selector (using `nuctl get fu -l <selector> `) or see all the labels per function in the wide view (using `nuctl get fu -o wide `)

## Function Templates and Reuse

Users can create a single function YAML file and create multiple functions from it each with different parameters by simply overriding the specific property using a command line flag (e.g. override environment variables).

instead of building the function code for every function instance we can build it once (using the cli `nuctl build` command), it will generate an artifact in a local or remote image repository, and we can use that artifact in multiple deployments and in different clusters (when using a shared repository).
