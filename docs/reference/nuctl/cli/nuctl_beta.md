## nuctl beta

A beta version of nuctl as a Nuclio api CLI client

### Options

```
      --access-key string        Access Key of a user with permissions to the nuclio API
      --api-url string           URL of the nuclio API (e.g. https://nuclio.io:8070)
  -h, --help                     help for beta
      --request-timeout string   Request timeout (default "60s")
      --skip-tls-verify          Skip TLS verification
      --username string          Username of a user with permissions to the nuclio API
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
* [nuctl beta redeploy](nuctl_beta_redeploy.md)	 - Redeploy one or more functions

