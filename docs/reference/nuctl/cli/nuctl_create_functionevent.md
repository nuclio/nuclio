## nuctl create functionevent

Create function events

```
nuctl create functionevent name [flags]
```

### Options

```
      --attrs string          JSON-encoded attributes for the function event (default "{}")
      --body string           body content to invoke the function with
      --display-name string   display name, if different than name (optional)
      --function string       function this event belongs to
  -h, --help                  help for functionevent
      --trigger-kind string   trigger kind to invoke (optional)
      --trigger-name string   trigger name to invoke (optional)
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

