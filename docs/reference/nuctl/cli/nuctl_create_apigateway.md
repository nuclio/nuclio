## nuctl create apigateway

Create api gateways

```
nuctl create apigateway name [flags]
```

### Options

```
      --attrs string                 JSON-encoded attributes for the api gateway (overrides all the rest) (default "{}")
      --authentication-mode string   Api gateway authentication mode. ['none', 'basicAuth', 'accessKey']
      --basic-auth-password string   The basic-auth password
      --basic-auth-username string   The basic-auth username
      --canary-function string       The api gateway canary function
      --canary-labels string         JSON-encoded custom labels for canary upstream of the api gateway (default "{}")
      --canary-percentage int        The canary function percentage
      --description string           Api gateway description
      --function string              The api gateway primary function
  -h, --help                         help for apigateway
      --host string                  Api gateway host address
      --labels string                JSON-encoded custom labels for the api gateway (default "{}")
      --path string                  Api gateway path (the URI that'll be concatenated to the host as an endpoint)
      --project string               The project the api gateway should be created in (default "project")
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

* [nuctl create](nuctl_create.md)	 - Create resources

