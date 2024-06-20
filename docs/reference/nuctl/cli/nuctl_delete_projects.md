## nuctl delete projects

(or project) Delete projects

```
nuctl delete projects name [flags]
```

### Options

```
  -h, --help                    help for projects
      --strategy string         Project deletion strategy; one of "restricted" (default), "cascading" (default "restricted")
      --wait                    Whether to wait until all project related resources are removed
      --wait-timeout duration   Wait timeout until all project related resources are removed (in conjunction with wait, default: 3m) (default 3m0s)
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

