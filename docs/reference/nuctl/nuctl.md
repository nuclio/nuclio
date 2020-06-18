# Nuctl

## About

Nuctl is Nuclio command-line interface that provide all nuclio-features from your terminal

## Download

To install nuctl all you need is to simply visit Nuclio [releases page](https://github.com/nuclio/nuclio/releases)
And download the matched binary to your platform (e.g.: `darwin` if you have `macOS`)

If you are adventurous enough, you can use the following snippet
```bash
curl -s https://api.github.com/repos/nuclio/nuclio/releases/latest \
			| grep -i "browser_download_url.*nuctl.*$(uname)" \
			| cut -d : -f 2,3 \
			| tr -d \" \
			| wget -O nuctl -qi - && chmod +x nuctl
```

## Use

Once downloaded, an informative help section is available for each command using `nuctl --help`

## Running platform

Nuctl will automatically identify its running platform, whether it is Docker or Kubernetes.

To ensure you run Nuctl against a specific platform, use `--platform kube` for Kubernetes or `--platform local` for docker

### Docker

Ensure your Docker daemon is running, run  `docker version` with the same user that will execute Nuctl commands

An example of function deployment using nuctl:

```bash
nuctl deploy helloworld \
    --path https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go
```

A function would be deployed, base on the source code given by the url and would be run as a docker container

> NOTE: You may change the path to a local file, directory or a compressed zip

To get more information about the function, run:

```bash
nuctl get function helloworld
```

An example output:
```bash
  NAMESPACE | NAME        | PROJECT | STATE | NODE PORT | REPLICAS  
  nuclio    | helloworld  | default | ready |     42089 | 1/1   
``` 

We can learn from the output that our deployed function `helloworld` is _running_ and using port `42089`.

Invoking the function is also possible using Nuctl by running:

```bash
nuctl invoke helloworld --method POST --body '{"hello":"world"}' --content-type "application/json"
```

An example output:

```bash
> Response headers:
Server = nuclio
Date = Thu, 18 Jun 2020 06:56:27 GMT
Content-Type = application/text
Content-Length = 21

> Response body:
Hello, from Nuclio :]
```

### Kubernetes

When running in Kubernetes, Nuctl would require you running a registry on your Kubernetes cluster and access to a `kubeconfig`

An example of function deployment using nuctl against a Kubernetes cluster can be found [here](/docs/setup/k8s/getting-started-k8s.md#deploy-a-function-with-the-nuclio-cli-nuctl)
