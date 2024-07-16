## nuctl

Nuclio command-line interface

### Options

```
      --concurrency int         Max number of parallel patches. The default value is equal to the number of CPUs. (default 4)
  -h, --help                    help for nuctl
  -k, --kubeconfig string       Path to a Kubernetes configuration file (admin.conf)
      --mask-sensitive-fields   Enable sensitive fields masking
  -n, --namespace string        Namespace
      --platform string         Platform identifier - "kube", "local", or "auto" (default "auto")
  -v, --verbose                 Verbose output
```

### SEE ALSO

* [nuctl beta](nuctl_beta.md)	 - A beta version of nuctl as a Nuclio api CLI client
* [nuctl build](nuctl_build.md)	 - Build a function
* [nuctl create](nuctl_create.md)	 - Create resources
* [nuctl delete](nuctl_delete.md)	 - Delete resources
* [nuctl deploy](nuctl_deploy.md)	 - Build and deploy a function, or deploy from an existing image
* [nuctl export](nuctl_export.md)	 - Export functions or projects
* [nuctl get](nuctl_get.md)	 - Display resource information
* [nuctl import](nuctl_import.md)	 - Import functions or projects
* [nuctl invoke](nuctl_invoke.md)	 - Invoke a function
* [nuctl parse](nuctl_parse.md)	 - Parse report
* [nuctl update](nuctl_update.md)	 - Update resources
* [nuctl version](nuctl_version.md)	 - Display the version number of the nuctl CLI

