## nuctl create

Create resources

### Options

```
  -h, --help   help for create
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
* [nuctl create apigateway](nuctl_create_apigateway.md)	 - Create api gateways
* [nuctl create functionevent](nuctl_create_functionevent.md)	 - Create function events
* [nuctl create project](nuctl_create_project.md)	 - Create a new project

