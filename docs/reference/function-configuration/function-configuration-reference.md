# Function-Configuration Reference

This document provides a reference of the Nuclio function configuration.

#### In This Document

- [Basic configuration structure](#basic-structure)
- [Function metadata (`metadata`)](#metadata)
- [Function Specification (`spec`)](#specification)
  - [Example](#spec-example)
- [See also](#see-also)

<a id="basic-structure"></a>
## Basic configuration structure

The basic structure of the Nuclio function configuration resembles Kubernetes resource definitions, and includes the `apiVersion`, `kind`, `metadata`, `spec`, and `status` sections. Following is an example of a minimal definition:

```yaml
apiVersion: "nuclio.io/v1"
kind: NuclioFunction
metadata:
  name: example
spec:
  image: example:latest
```

<a id="metadata"></a>
## Function Metadata (`metadata`)

The `metadata` section includes the following attributes:

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| name | string | The name of the function |
| namespace | string | A level of isolation provided by the platform (e.g., Kubernetes) |
| labels | map | A list of key-value tags that are used for looking up the function (immutable, can't update after first deployment) |
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

<a id="specification"></a>
## Function Specification (`spec`)

The `spec` section contains the requirements and attributes and has the following elements:

| **Path** | **Type** | **Description** |
| :--- | :--- | :--- |
| description | string | A textual description of the function |
| handler | string | The entry point to the function, in the form of `package:entrypoint`; varies slightly between runtimes, see the appropriate runtime documentation for specifics |
| runtime | string | The name of the language runtime - `golang` \| `python:3.6` \| `python:3.7` \| `python:3.8` \| `python:3.9` \| `shell` \| `java` \| `nodejs` | 
| <a id="spec.image"></a>image | string | The name of the function's container image &mdash; used for the `image` [code-entry type](#spec.build.codeEntryType); see [Code-Entry Types](/docs/reference/function-configuration/code-entry-types.md#code-entry-type-image) |
| env | map | A name-value environment-variables tuple; it's also possible to reference secrets from the map elements, as demonstrated in the [specifcation example](#spec-example) |
| volumes | map | A map in an architecture similar to Kubernetes volumes, for Docker deployment |
| replicas | int | The number of desired instances; 0 for auto-scaling. |
| minReplicas | int | The minimum number of replicas |
| platform.attributes.restartPolicy.name | string | The name of the restart policy for the function-image container; applicable only to Docker platforms |
| platform.attributes.restartPolicy.maximumRetryCount | int | The maximum retries for restarting the function-image container; applicable only to Docker platforms |
| platform.attributes.mountMode | string | Function mount mode, which determines how Docker mounts the function configurations - `bind` \| `volume` (default: `bind`); applicable only to Docker platforms |
| maxReplicas | int | The maximum number of replicas |
| targetCPU | int | Target CPU when auto scaling, as a percentage (default: 75%) |
| dataBindings | See reference | A map of data sources used by the function ("data bindings") |
| triggers.(name).maxWorkers | int | The max number of concurrent requests this trigger can process |
| triggers.(name).kind | string | The trigger type (kind) - `cron` \| `eventhub` \| `http` \| `kafka-cluster` \| `kinesis` \| `nats` \| `rabbit-mq` |
| triggers.(name).url | string | The trigger specific URL (not used by all triggers) |
| triggers.(name).annotations | list of strings | Annotations to be assigned to the trigger, if applicable |
| triggers.(name).workerAvailabilityTimeoutMilliseconds | int | The number of milliseconds to wait for a worker if one is not available. 0 = never wait (default: 10000, which is 10 seconds)|
| triggers.(name).attributes | See [reference](/docs/reference/triggers) | The per-trigger attributes |
| <a id="spec.build.path"></a>build.path | string | The URL of a GitHub repository or an archive-file that contains the function code &mdash; for the `git`, `github` or `archive` [code-entry type](#spec.build.codeEntryType) &mdash; or the URL of a function source-code file; see [Code-Entry Types](/docs/reference/function-configuration/code-entry-types.md) |
| <a id="spec.build.functionSourceCode"></a>build.functionSourceCode | string | Base-64 encoded function source code for the `sourceCode` [code-entry type](#spec.build.codeEntryType); see [Code-Entry Types](/docs/reference/function-configuration/code-entry-types.md#code-entry-type-sourcecode) |
| build.registry | string | The container image repository to which the built image will be pushed |
| build.noBaseImagePull | string | Do not pull any base images when building, use local images only |
| build.noCache | string | Do not use any caching when building container images |
| build.baseImage | string | The name of a base container image from which to build the function's processor image |
| build.Commands | list of string | Commands run opaquely as part of container image build |
| build.onbuildImage | string | The name of an "onbuild" container image from which to build the function's processor image; the name can include `{{ .Label }}` and `{{ .Arch }}` for formatting |
| build.image | string | The name of the built container image (default: the function name) |
| <a id="spec.build.codeEntryType"></a>build.codeEntryType | string | The function's code-entry type - `archive` \| `git` \| `github` \| `image` \| `s3` \| `sourceCode`; see [Code-Entry Types](/docs/reference/function-configuration/code-entry-types.md) |
| <a id="spec.build.codeEntryAttributes"></a>build.codeEntryAttributes | See [reference](/docs/reference/function-configuration/code-entry-types.md#external-func-code-entry-types) | Code-entry attributes, which provide information for downloading the function when using the `github`, `s3`, or `archive` [code-entry type](#spec.build.codeEntryType) |
| runRegistry | string | The container image repository from which the platform will pull the image |
| runtimeAttributes | See [reference](/docs/reference/runtimes/) | Runtime-specific attributes |
| resources | See [reference](https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/) | Limit resources allocated to deployed function |
| readinessTimeoutSeconds | int | Number of seconds that the controller will wait for the function to become ready before declaring failure (default: 60) |
| waitReadinessTimeoutBeforeFailure | bool | Wait for the expiration of the readiness timeout period even if the deployment fails or isn't expected to complete before the readinessTimeout expires |
| avatar | string | Base64 representation of an icon to be shown in UI for the function |
| eventTimeout | string | Global event timeout, in the format supported for the `Duration` parameter of the [`time.ParseDuration`](https://golang.org/pkg/time/#ParseDuration) Go function |
| securityContext.runAsUser | int | The user ID (UID) for runing the entry point of the container process |
| securityContext.runAsGroup | int | The group ID (GID) for running the entry point of the container process |
| securityContext.fsGroup | int | A supplemental group to add and use for running the entry point of the container process |
| serviceType | string | Describes ingress methods for a service |
| affinity | v1.Affinity | Set of rules used to determine the node that schedule the pod |
| nodeSelector | map | Constrain function pod to a node by key-value pairs selectors |
| nodeName | string | Constrain function pod to a node by node name |
| priorityClassName | string | Indicates the importance of a function pod relatively to other function pods |
| preemptionPolicy | string | Function pod preemption policy (one of `Never` or `PreemptLowerPriority`) |
| tolerations | []v1.Toleration  | Function pod tolerations |

<a id="spec-example"></a>
### Example

```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  image: myfunctionimage:latest
  platform:
    attributes:

      # Docker will retry starting the function's image container 3 times.
      # For more information, see https://docs.docker.com/config/containers/start-containers-automatically.
      restartPolicy:
        name: on-failure
        maximumRetryCount: 3

      # Use `volume` to mount the processor into the function.
      # For more information, see https://docs.docker.com/storage/volumes.
      mountMode: volume
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
  securityContext:
    runAsUser: 1000
    runAsGroup: 2000
    fsGroup: 3000
```

## See also

- [Deploying Functions](/docs/tasks/deploying-functions.md)
- [Code-Entry Types](/docs/reference/function-configuration/code-entry-types.md)
