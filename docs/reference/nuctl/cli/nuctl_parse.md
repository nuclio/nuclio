## nuctl parse

Parse report

```
nuctl parse [options] [flags]
```

### Options

```
      --failed                    Show only failures
  -h, --help                      help for parse
      --output-path string        File path to save the parsed report
      --report-file-path string   Path to import report (default "nuctl-import-report.json")
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

