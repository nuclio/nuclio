# Configuring a Function

There are often cases in which providing code is not enough to deploy a function. For example, if

- The function expects environment variables or secrets.
- You would like to trigger the function through Kafka, Kinesis, or a similar tool, which requires configuration to connect to the data source.
- There are third-party dependencies or additional files (both language packages and OS) that need to reside alongside the function.

For such cases, and many others, you need to provide a function configuration alongside your function code. nuclio provides you with several mechanisms for providing the function configuration:

- A **function.yaml** file.
- Inline configuration by means of crafting a comment in your code that contains the **function.yaml** contents.
- Command-line arguments for the nuclio CLI (`nuctl`).  
  Such argument will override the **function.yaml** configuration, if present.
- The playground UI, through the **Configuration** tab.

While there are several mechanisms to provide the configuration, there is only one configuration schema.

## Configuration schema

The basic structure resembles Kubernetes resource definitions, and includes the `apiVersion`, `kind`, `metadata`, `spec`, and `status` sections. Following is an example if a minimal definition:

```yaml
apiVersion: "nuclio.io/v1"
kind: Function
metadata:
  name: example
spec:
  image: example:latest
```

## Metadata

The `metadata` section includes the following attributes:

- **name**: The name of the function.
- **namespace**: The kubernetes namespace. This namespace can be viewed as an independent project.
- **labels**: A list of key-value tags that are used for looking up the function. Note that `function name`, `version`, and `alias` are reserved and filled automatically by the controller.
- **annotations**: A list of annotations based on the key-value tags.

## Requirement spec

The `spec` section contains the requirements and attributes and has the following elements:

- **description** (string): A textual description of the function.
- **handler** (string): The name of the function handler to call. If not set, nuclio will try to deduce this value automatically.
- **runtime** (string): The name of the language runtime. If not set, nuclio will try to deduce this value automatically.
- **code**: a structure containing the source code or its location of and access credentials.
- **image** (string): The full path to the function's artifact (container image). Note that you can provide either the image source code or a prebuilt image, but not both.
- **env**: A name-value environment-variable tuple. It is also possible to point to secrets, as demonstrated in the following example.
- **resources**: Specify the requested CPU and memory resources and their limits  (similar to the Kubernetes pod resources definition).
- **replicas** (int): The number of desired instances; 0 for auto-scaling.
- **minReplicas** (int): The minimum number of replicas.
- **maxReplicas** (int): The maximum number of replicas.
- **disable** (Boolean): Set to `True` to disable a function.
- **dataBindings**: A list of data resources used by the function ("data bindings"). The bindings are currently limited to the iguazio data platform resources.
- **triggers**: A list of event sources and their configurations (see examples below). The trigger name must be unique per function and all its versions. (In the future, it will be possible to move triggers between versions or have the same trigger feed multiple function versions for canary deployments.)
- **build**: A configuration that is passed to the builder, which includes information such as base image and on-build commands for dependency installations.

> Note: Other elements are not fully supported yet, Additional elements will be documented as they become available.

When creating a function using the CLI `deploy` command, each of the elements described above can be specified or overwritten using a command-line argument. Run `nuctl deploy --help` for details.

## Complete example (YAML)

```yaml
apiVersion: "nuclio.io/v1"
kind: Function
metadata:
  name: example
  namespace: myproject
  labels:
    author: joe
spec:
  runtime: golang
  handler: Handler
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
    http:
      maxWorkers: 4
      kind: "http"
      attributes:

        # see "Invoking Functions By Name With Kubernetes Ingresses" for more details
        # on configuring ingresses
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
      url: "10.0.0.3:4222"
      attributes:
        "topic": "my.topic"

  dataBindings:
    db0:
      class: "v3io"
      secret: "something"
      url: "http://192.168.51.240:8081/1024"


```

The example above demonstrates how you can use namespaces, specify labels, use environment variables or secrets, and specify exact memory and CPU resources. For the example to work in Kubernetes, the namespace `myproject` and the secret `my-secret` must be defined in advance.

> Note: When specifying labels, you can list functions based on a specific label selector by using the CLI command `nuctl get fu -l <selector>`, or view all the labels per function in the wide view by using the command `nuctl get fu -o wide`.

## Function templates and reuse

You can create a single function YAML file and create multiple functions from this file, each with different parameters, by simply overriding the specific property using a command-line flag (for example, override environment variables).

Instead of building the function code for every function instance, you can build it once by using the `nuctl build` CLI command. This command generates an artifact in a local or remote image repository. You can use this artifact in multiple deployments and in different clusters (when using a shared repository).

