## nuctl export functions

(or function) Export functions

### Synopsis

(or function) Export the configuration of a specific function or of all deployed Nuclio functions (default)
to the standard output, in JSON or YAML format (see -o|--output)

Arguments:
  <function> (string) The name of a function to export

```
nuctl export functions [<function>] [flags]
```

### Options

```
  -h, --help            help for functions
  -o, --output string   Output format - "json" or "yaml" (default "yaml")
```

### Options inherited from parent commands

```
      --cleanup-spec            Clean up the image info from the function spec
      --concurrency int         Max number of parallel patches. The default value is equal to the number of CPUs. (default 4)
  -k, --kubeconfig string       Path to a Kubernetes configuration file (admin.conf)
      --mask-sensitive-fields   Enable sensitive fields masking
  -n, --namespace string        Namespace
      --no-scrub                Export all function data, including sensitive and unnecessary data
      --platform string         Platform identifier - "kube", "local", or "auto" (default "auto")
  -v, --verbose                 Verbose output
```

### SEE ALSO

* [nuctl export](nuctl_export.md)	 - Export functions or projects

