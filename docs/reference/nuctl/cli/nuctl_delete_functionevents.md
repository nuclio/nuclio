## nuctl delete functionevents

(or functionevent) Delete function event

```
nuctl delete functionevents name [flags]
```

### Options

```
  -h, --help   help for functionevents
```

### Options inherited from parent commands

```
      --concurrency int         Max number of parallel patches. The default value is equal to the number of CPUs. (default 4)
  -f, --force                   Force delete resources
  -k, --kubeconfig string       Path to a Kubernetes configuration file (admin.conf)
      --mask-sensitive-fields   Enable sensitive fields masking
  -n, --namespace string        Namespace
      --platform string         Platform identifier - "kube", "local", or "auto" (default "auto")
  -v, --verbose                 Verbose output
```

### SEE ALSO

* [nuctl delete](nuctl_delete.md)	 - Delete resources

