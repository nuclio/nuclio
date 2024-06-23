## nuctl build

Build a function

```
nuctl build function-name [options] [flags]
```

### Options

```
      --base-image string               Name of the base image (default - per-runtime default)
      --build-code-entry-attrs string   JSON-encoded build code entry attributes for the function (default "{}")
      --build-command String            Commands to run when building the processor image
      --build-runtime-attrs string      JSON-encoded build runtime attributes for the function (default "{}")
      --code-entry-type string          Type of code entry (for example, "url", "github", "image")
  -f, --file string                     Path to a function-configuration file
      --handler string                  Name of a function handler
  -h, --help                            help for build
  -i, --image string                    Name of a container image (default - the function name)
      --no-cleanup                      Don't clean up temporary directories
      --no-pull                         Don't pull base images - use local versions
      --offline                         Don't assume internet connectivity exists
      --onbuild-image string            The runtime onbuild image used to build the processor image
      --output-image-file string        Path to output container image of the build
  -p, --path string                     Path to the function's source code
  -r, --registry string                 URL of a container registry (env: NUCTL_REGISTRY)
      --runtime string                  Runtime (for example, "golang", "python:3.9")
      --source string                   The function's source code (overrides "path")
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

