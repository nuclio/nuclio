## nuctl beta redeploy

Redeploy one or more functions

### Synopsis


Redeploy one or more functions. If no function name is specified, 
all functions in the namespace will be redeployed.

Note: This command works on functions that were previously deployed, or imported functions.
	  To deploy a new function, use the 'deploy' command.

Arguments:
  <function> (string) The name of a function to redeploy. Can be specified multiple times.

```
nuctl beta redeploy [<function>] [flags]
```

### Options

```
      --desired-state string         Desired function state ["ready", "scaledToZero"] (default "ready")
      --exclude-functions strings    Exclude functions to patch
      --exclude-functions-with-gpu   Skip functions with GPU
      --exclude-projects strings     Exclude projects to patch
      --from-report                  Redeploy failed and retryable functions from the given report file (if arguments are also given, they will be redeployed as well)
  -h, --help                         help for redeploy
      --imported-only                Deploy only imported functions
      --report-file-path string      Path to redeployment report (default "nuctl-redeployment-report.json")
      --save-report                  Save redeployment report to a file
      --verify-external-registry     verify registry is external
  -w, --wait                         Wait for function deployment to complete
      --wait-timeout duration        Wait timeout duration for the function deployment, e.g 30s, 5m (default 15m0s)
```

### Options inherited from parent commands

```
      --access-key string        Access Key of a user with permissions to the nuclio API
      --api-url string           URL of the nuclio API (e.g. https://nuclio.io:8070)
      --concurrency int          Max number of parallel patches. The default value is equal to the number of CPUs. (default 4)
  -k, --kubeconfig string        Path to a Kubernetes configuration file (admin.conf)
      --mask-sensitive-fields    Enable sensitive fields masking
  -n, --namespace string         Namespace
      --platform string          Platform identifier - "kube", "local", or "auto" (default "auto")
      --request-timeout string   Request timeout (default "60s")
      --skip-tls-verify          Skip TLS verification
      --username string          Username of a user with permissions to the nuclio API
  -v, --verbose                  Verbose output
```

### SEE ALSO

* [nuctl beta](nuctl_beta.md)	 - A beta version of nuctl as a Nuclio api cli client

