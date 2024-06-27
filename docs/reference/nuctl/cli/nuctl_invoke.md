## nuctl invoke

Invoke a function

```
nuctl invoke function-name [flags]
```

### Options

```
  -b, --body string           HTTP message body
  -c, --content-type string   HTTP Content-Type
      --external-ips string   External IP addresses (comma-delimited) with which to invoke the function
  -d, --headers string        HTTP headers (name=val1[,name=val2,...])
  -h, --help                  help for invoke
  -l, --log-level string      Log level - "none", "debug", "info", "warn", or "error" (default "info")
  -m, --method string         HTTP method for invoking the function
  -p, --path string           Path to the function to invoke
      --raise-on-status       Fail nuctl in case function invocation returns non-200 status code
      --skip-tls              Skip TLS verification
  -t, --timeout duration      Invocation request timeout (default 1m0s)
      --via string            Invoke the function via - "any": a load balancer or an external IP; "loadbalancer": a load balancer; "external-ip": an external IP (default "any")
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

