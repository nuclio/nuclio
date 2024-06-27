## nuctl get apigateways

(or apigateway) Display api gateways information

```
nuctl get apigateways name [flags]
```

### Options

```
      --function-name string   Function name to filter api gateways
  -h, --help                   help for apigateways
  -o, --output string          Output format - "text", "wide", "yaml", or "json" (default "text")
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

* [nuctl get](nuctl_get.md)	 - Display resource information

