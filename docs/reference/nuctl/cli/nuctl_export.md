## nuctl export

Export functions or projects

### Synopsis

Export the configuration of a specific function or project or of all functions/projects (default)
to the standard output, in JSON or YAML format

### Options

```
      --cleanup-spec   Clean up the image info from the function spec
  -h, --help           help for export
      --no-scrub       Export all function data, including sensitive and unnecessary data
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
* [nuctl export functions](nuctl_export_functions.md)	 - (or function) Export functions
* [nuctl export projects](nuctl_export_projects.md)	 - (or project) Export projects (including all functions, function events, and API gateways)

