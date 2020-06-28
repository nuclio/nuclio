# nuctl &mdash; The Nuclio CLI

#### In this document

- [About](#about)
- [Download](#download)
- [Usage](#usage)
- [Running platform](#running-platform)
- [Docker](#docker)
- [Kubernetes](#kubernetes)

## About

`nuctl` is Nuclio's command-line interface that provides you with all the nuclio features, accessible from your terminal

## Download

To install `nuctl` all you need to do, is simply visit Nuclio [releases page](https://github.com/nuclio/nuclio/releases)
and download the binary appropriate to your platform (e.g.: `darwin` if you have `macOS`)

You can use the following bash snippet to download the latest `nuctl` release
```bash
curl -s https://api.github.com/repos/nuclio/nuclio/releases/latest \
			| grep -i "browser_download_url.*nuctl.*$(uname)" \
			| cut -d : -f 2,3 \
			| tr -d \" \
			| wget -O nuctl -qi - && chmod +x nuctl
```

## Usage

Once downloaded, an informative help section is available using `nuctl --help`

## Running platform

`nuctl` will automatically identify its running platform, whether it is Docker or Kubernetes.

To ensure you run `nuctl` against a specific platform, use `--platform kube` for Kubernetes or `--platform local` for docker

### Docker

An example of function deployment using `nuctl` against Docker can be found [here](/docs/setup/docker/getting-started-docker.md)

### Kubernetes

When running in Kubernetes, `nuctl` would require you running a registry on your Kubernetes cluster and access to a `kubeconfig`

An example of function deployment using `nuctl` against a Kubernetes cluster can be found [here](/docs/setup/k8s/getting-started-k8s.md#deploy-a-function-with-the-nuclio-cli-nuctl)
