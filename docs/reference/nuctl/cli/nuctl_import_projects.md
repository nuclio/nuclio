## nuctl import projects

(or project) Import projects (including all functions, function events, and API gateways)

### Synopsis

Import the configurations of one or more projects (including all project 
functions, function events, and API gateways) from a configurations file
or from standard input (default)

Note: The command doesn't deploy the functions in the  imported projects.
      To deploy an imported function, use the 'deploy' command.

Arguments:
  <config file> (string) Path to a project-configurations file in JSON or YAML format (see -o|--output).
                         If not provided, the configuration is imported from standard input (stdin).

```
nuctl import projects [<config file>] [flags]
```

### Options

```
  -h, --help                          help for projects
      --skip strings                  Names of projects to skip (don't import), as a comma-separated list
      --skip-label-selectors string   Kubernetes label-selectors filter that identifies projects to skip (don't import)
```

### Options inherited from parent commands

```
      --concurrency int           Max number of parallel patches. The default value is equal to the number of CPUs. (default 4)
  -k, --kubeconfig string         Path to a Kubernetes configuration file (admin.conf)
      --mask-sensitive-fields     Enable sensitive fields masking
  -n, --namespace string          Namespace
      --platform string           Platform identifier - "kube", "local", or "auto" (default "auto")
      --report-file-path string   Path to import report (default "nuctl-import-report.json")
      --save-report               Save importing report to a file
      --skip-autofix              Skip config autofix if error occurred
  -v, --verbose                   Verbose output
```

### SEE ALSO

* [nuctl import](nuctl_import.md)	 - Import functions or projects

