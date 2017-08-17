# Function Configuration and Metadata
nuclio function configuration and metadata is described in the function spec, a user can create the spec as a YAML or JSON file, push it through the API, or use the CLI to define or override one.

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
 - **dataBindings** - describe a list of data resources used by the function
 - **disable** (boolean) - can be set to True to disable a function

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
```

the example above demonstrates how we can use Kubernetes namespaces, specify labels, use environment variables and secrets, and specify exact memory and cpu resources. For the example to work the Kubernetes namespace `myproject` and the secret `my-secret` must be defined ahead of time.

**Note:** when specifying labels you can list functions based on a specific label selector (using `nuctl get fu -l <selector> `) or see all the labels per function in the wide view (using `nuctl get fu -o wide `)

## Function Templates and Reuse

Users can create a single function YAML file and create multiple functions from it each with different parameters by simply overriding the specific property using a command line flag (e.g. override environment variables).

instead of building the function code for every function instance we can build it once (using the cli `nuctl build` command), it will generate an artifact in a local or remote image repository, and we can use that artifact in multiple deployments and in different clusters (when using a shared repository).

