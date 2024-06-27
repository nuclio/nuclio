## nuctl delete

Delete resources

### Options

```
  -f, --force   Force delete resources
  -h, --help    help for delete
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
* [nuctl delete apigateways](nuctl_delete_apigateways.md)	 - (or apigateway) Delete api gateway
* [nuctl delete functionevents](nuctl_delete_functionevents.md)	 - (or functionevent) Delete function event
* [nuctl delete functions](nuctl_delete_functions.md)	 - (or function) Delete functions
* [nuctl delete projects](nuctl_delete_projects.md)	 - (or project) Delete projects

