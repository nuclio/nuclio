## nuctl create project

Create a new project

### Synopsis

Create a new Nuclio project.

```
nuctl create project name [flags]
```

### Options

```
      --description string   Project description
  -h, --help                 help for project
      --owner string         Project owner
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

