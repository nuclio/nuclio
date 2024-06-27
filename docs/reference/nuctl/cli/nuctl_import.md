## nuctl import

Import functions or projects

### Synopsis

Import the configurations of one or more functions or projects
from a configuration file or from the standard input (default)

### Options

```
  -h, --help                      help for import
      --report-file-path string   Path to import report (default "nuctl-import-report.json")
      --save-report               Save importing report to a file
      --skip-autofix              Skip config autofix if error occurred
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
* [nuctl import functions](nuctl_import_functions.md)	 - (or function) Import functions
* [nuctl import projects](nuctl_import_projects.md)	 - (or project) Import projects (including all functions, function events, and API gateways)

