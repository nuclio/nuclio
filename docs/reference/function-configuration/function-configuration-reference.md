# Function configuration reference

This document describes the various fields in the function configuration.

## Basic structure 

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

| Path | Type | Description | 
| --- | --- | --- |  
| name | string | The name of the function |
| namespace | string | A level of isolation provided by the platform (e.g. Kubernetes) |
| labels | map | A list of key-value tags that are used for looking up the function |
| annotations | map | A list of annotations based on the key-value tags |

#### Example

```yaml
metadata:
  name: example
  namespace: nuclio
  labels:
    l1: lv1
    l2: lv2
    l3: 100
  annotations:
    a1: av1  
```

## Specification

The `spec` section contains the requirements and attributes and has the following elements:

| Path | Type | Description | 
| --- | --- | --- |  
| description | string | A textual description of the function |
| handler | string | The entrypoint to the function, in the form of `package:entrypoint`. Varies slightly between runtimes, see the appropriate runtime documentation for specifics |
| runtime | string | The name of the language runtime. One of: `golang`, `python:2.7`, `python:3.6`, `shell`, `java`, `nodejs`, `pypy` | 
| image | string | The container image holding the function |
| env | map | A name-value environment-variable tuple. It is also possible to point to secrets, as demonstrated in the following example |
| replicas | int | The number of desired instances; 0 for auto-scaling. |
| minReplicas | int | The minimum number of replicas |
| maxReplicas | int | The maximum number of replicas |
| dataBindings | See REF | A map of data sources used by the function ("data bindings") |
| triggers.(name).maxWorkers | int | The max number of concurrent requests this trigger can process |
| triggers.(name).kind | string | The kind of trigger. One of `http`, `kafka`, `kinesis`, `eventhub`, `cron`, `nats`, `rabbitmq` |
| triggers.(name).url | string | The trigger specific URL (not used by all triggers) |
| triggers.(name).attributes | [See reference](/docs/reference/triggers) | The per-trigger attributes |
| build.path | string | A local directory or URL to a file/archive containing source and configuration |
| build.functionSourceCode | string | The source code of the function, encoded in Base64. Mutually exclusive with build.path |
| build.registry | string | The container image repository to which the built image will be pushed |
| build.noBaseImagePull | string | Do not pull any base images when building, use local images only |
| build.noCache | string | Do not use any caching when building container images |
| build.baseImage | string | Currently one of "alpine" or "jessie" |
| build.Commands | list of string | Commands run opaquely as part of container image build |
| runRegistry | string | The container image repository from which the platform will pull the image |
| runtimeAttributes | See REF | Runtime specific attributes, see runtime documentation for specifics |

```yaml
spec:
  runtime: golang
  handler: main:Handler
  image: myfunctionimage:latest
  replicas: 1
  env:
  - name: SOME_ENV
    value: abc
  - name: SECRET_PASSWORD_ENV_VAR
    valueFrom:
      secretKeyRef:
        name: my-secret
        key: password
  build:
    registry: localhost:5000
    noBaseImagePull: true
    noCache: true
    commands:
    - apk --update --no-cache add curl
    - pip install simplejson
```
