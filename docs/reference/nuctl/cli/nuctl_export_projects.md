## nuctl export projects

(or project) Export projects (including all functions, function events, and API gateways)

### Synopsis

(or project) Export the configuration of a specific project (including
all its functions, function events, and API gateways) or of all projects (default)
to the standard output, in JSON or YAML format (see -o|--output)

Arguments:
  <project> (string) The name of a project to export

```
nuctl export projects [<project>] [flags]
```

### Options

```
  -h, --help            help for projects
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

