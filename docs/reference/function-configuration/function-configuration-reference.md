# Function-Configuration Reference

This document describes the various fields in the function configuration.

#### In This Document

- [Basic structure](#basic-structure)
- [Metadata](#metadata)
- [Specification](#specification)
- [See also](#see-also)

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
| :--- | :--- | :--- |
| name | string | The name of the function |
| namespace | string | A level of isolation provided by the platform (e.g., Kubernetes) |
| labels | map | A list of key-value tags that are used for looking up the function |
| annotations | map | A list of annotations based on the key-value tags |

### Example

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
| :--- | :--- | :--- |
| description | string | A textual description of the function |
| handler | string | The entry point to the function, in the form of `package:entrypoint`. Varies slightly between runtimes, see the appropriate runtime documentation for specifics |
| runtime | string | The name of the language runtime. One of: `golang`, `python:2.7`, `python:3.6`, `shell`, `java`, `nodejs`, `pypy` | 
| image | string | The container image holding the function |
| env | map | A name-value environment-variable tuple. It is also possible to point to secrets, as demonstrated in the following example |
| volumes | map | A map in an architecture similar to k8s volumes, for docker deployment |
| replicas | int | The number of desired instances; 0 for auto-scaling. |
| minReplicas | int | The minimum number of replicas |
| maxReplicas | int | The maximum number of replicas |
| targetCPU | int | Target CPU when auto-scaling, in percentage. Defaults to 75% |
| dataBindings | See reference | A map of data sources used by the function ("data bindings") |
| triggers.(name).maxWorkers | int | The max number of concurrent requests this trigger can process |
| triggers.(name).kind | string | The kind of trigger. One of `http`, `kafka`, `kinesis`, `eventhub`, `cron`, `nats`, `rabbitmq` |
| triggers.(name).url | string | The trigger specific URL (not used by all triggers) |
| triggers.(name).attributes | See [reference](/docs/reference/triggers) | The per-trigger attributes |
| build.path | string | A local directory or URL to a file/archive containing source and configuration |
| build.functionSourceCode | string | The source code of the function, encoded in Base64. Mutually exclusive with build.path |
| build.registry | string | The container image repository to which the built image will be pushed |
| build.noBaseImagePull | string | Do not pull any base images when building, use local images only |
| build.noCache | string | Do not use any caching when building container images |
| build.baseImage | string | The base image from which the processor image will be built from |
| build.Commands | list of string | Commands run opaquely as part of container image build |
| build.onbuildImage | string | Specifies the "Onbuild" image from which the processor image will be built from. Can use {{ .Label }} and {{ .Arch }} for formatting |
| runRegistry | string | The container image repository from which the platform will pull the image |
| runtimeAttributes | See reference | Runtime specific attributes, see runtime documentation for specifics |
| resources | See [reference](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/) | Limit resources allocated to deployed function |

### Example

```yaml
spec:
  description: my Golang function
  handler: main:Handler
  runtime: golang
  image: myfunctionimage:latest
  env:
  - name: SOME_ENV
    value: abc
  - name: SECRET_PASSWORD_ENV_VAR
    valueFrom:
      secretKeyRef:
        name: my-secret
        key: password
  volumes:
    - volume:
        hostPath:
          path: "/var/run/docker.sock"
      volumeMount:
        mountPath: "/var/run/docker.sock"
  minReplicas: 2
  maxReplicas: 8
  targetCPU: 60
  build:
    registry: localhost:5000
    noBaseImagePull: true
    noCache: true
    commands:
    - apk --update --no-cache add curl
    - pip install simplejson
  resources:
    requests:
      cpu: 1
      memory: 128M
    limits:
      cpu: 2
      memory: 256M  
```

## See also

- [Deploying Functions](/docs/tasks/deploying-functions.md)

