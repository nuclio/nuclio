## nuctl deploy

Build and deploy a function, or deploy from an existing image

```
nuctl deploy function-name [flags]
```

### Options

```
      --annotations string                 Additional function annotations (ant1=val1[,ant2=val2,...])
      --base-image string                  Name of the base image (default - per-runtime default)
      --build-code-entry-attrs string      JSON-encoded build code entry attributes for the function (default "{}")
      --build-command String               Commands to run when building the processor image
      --build-runtime-attrs string         JSON-encoded build runtime attributes for the function (default "{}")
      --code-entry-type string             Type of code entry (for example, "url", "github", "image")
      --data-bindings string               JSON-encoded data bindings for the function
      --desc string                        Function description
  -d, --disable                            Start the function as disabled (don't run yet)
  -e, --env String                         Environment variables env1=val1
  -f, --file string                        Path to a function-configuration file
      --fsgroup int                        Run function process with supplementary groups (default -1)
      --handler string                     Name of a function handler
  -h, --help                               help for deploy
      --http-trigger-service-type string   A Kubernetes ServiceType to apply to the HTTP trigger
  -i, --image string                       Name of a container image (default - the function name)
      --input-image-file string            Path to an input function-image Docker archive file
  -l, --labels string                      Additional function labels (lbl1=val1[,lbl2=val2,...])
      --logger-level string                One of debug, info, warn, error. By default, uses platform configuration
      --max-replicas int                   Maximal number of function replicas (default -1)
      --min-replicas int                   Minimal number of function replicas (default -1)
      --no-cleanup                         Don't clean up temporary directories
      --no-pull                            Don't pull base images - use local versions
      --nodeName string                    Run function pod on a Node by name-matching selection constrain
      --nodeSelector string                Run function pod on a Node by key=value selection constraints (key1=val1[,key2=val2,...])
      --offline                            Don't assume internet connectivity exists
      --onbuild-image string               The runtime onbuild image used to build the processor image
  -p, --path string                        Path to the function's source code
      --platform-config string             JSON-encoded platform specific configuration
      --preemptionPolicy string            Function pod preemption policy
      --priorityClassName string           Indicates the importance of a function Pod relatively to other function pods
      --project-name string                The name of the function's parent project
      --publish                            Publish the function
      --readiness-timeout int              Maximum wait period for the function to be ready, in seconds (default -1)
  -r, --registry string                    URL of a container registry (env: NUCTL_REGISTRY)
      --replicas int                       Set to any non-negative integer to use a static number of replicas (default -1)
      --resource-limit String              Resource restrictions of the format '<resource name>=<quantity>' (for example, 'cpu=3')
      --resource-request String            Requested resources of the format '<resource name>=<quantity>' (for example, 'cpu=3')
      --run-as-group int                   Run function process with group ID (default -1)
      --run-as-user int                    Run function process with user ID (default -1)
      --run-image string                   Name of an existing image to deploy (default - build a new image to deploy)
      --run-registry string                URL of a registry for pulling the image, if differs from -r/--registry (env: NUCTL_RUN_REGISTRY)
      --runtime string                     Runtime (for example, "golang", "python:3.9")
      --runtime-attrs string               JSON-encoded runtime attributes for the function
      --source string                      The function's source code (overrides "path")
      --target-cpu int                     Target CPU-usage percentage when auto-scaling (default -1)
      --triggers string                    JSON-encoded triggers for the function
      --volume String                      Volumes for the deployment function (src1=dest1[,src2=dest2,...])
```

### Options inherited from parent commands

```
      --concurrency int         Max number of parallel patches. The default value is equal to the number of CPUs. (default 4)
  -k, --kubeconfig string       Path to a Kubernetes configuration file (admin.conf)
      --mask-sensitive-fields   Enable sensitive fields masking
  -n, --namespace string        Namespace
      --platform string         Platform identifier - "kube", "local", or "auto" (default "auto")
  -v, --verbose                 Verbose output
```

### SEE ALSO

* [nuctl](nuctl.md)	 - Nuclio command-line interface

