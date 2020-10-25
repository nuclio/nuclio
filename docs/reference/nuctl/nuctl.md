# The Nuclio CLI (nuctl)

#### In this document

- [Overview](#overview)
- [Download and Installation](#download-n-install)
- [Usage](#usage)
- [The running platform](#running-platform)
  - [Local Docker](#docker)
  - [Kubernetes](#kubernetes)

<a id="overview"></a>
## Overview

`nuctl` is Nuclio's command-line interface (CLI), which provides command-line access to Nuclio features.

<a id="download-n-install"></a>
## Download and Installation

To install `nuctl`, simply visit the Nuclio [releases page](https://github.com/nuclio/nuclio/releases) and download the appropriate CLI binary for your environment (for example, **nuctl-&lt;version&gt;-darwin-amd64** for a machine running macOS).

You can use the following command to download the latest `nuctl` release:
```sh
curl -s https://api.github.com/repos/nuclio/nuclio/releases/latest \
			| grep -i "browser_download_url.*nuctl.*$(uname)" \
			| cut -d : -f 2,3 \
			| tr -d \" \
			| wget -O nuctl -qi - && chmod +x nuctl
```

<a id="usage"></a>
## Usage

After you download `nuctl`, use the `--help` option to see full usage instructions:
```sh
nuctl --help
```

<a id="running-platform"></a>
## The running platform

`nuctl` automatically identifies the platform from which it's being run.
You can also use the `--platform` CLI flag to ensure that you're running against a specific platform, as explained in this reference:

- [Local Docker](#docker)
- [Kubernetes](#kubernetes)

<a id="docker"></a>
### Local Docker

To force `nuctl` to run locally, using a Docker daemon, add `--platform local` to the CLI command.

For an example of function deployment using `nuctl` against Docker, see the Nuclio [Docker getting-started guide](/docs/setup/docker/getting-started-docker.md).

<a id="kubernetes"></a>
### Kubernetes

To force `nuctl` to run against a Kubernetes instance of Nuclio, add `--platform kube` to the CLI command.

When running on Kubernetes, `nuctl` requires a running registry on your Kubernetes cluster and access to a kubeconfig file.

For an example of function deployment using `nuctl` against a Kubernetes cluster, see the Nuclio [Kubernetes getting-started guide](/docs/setup/k8s/getting-started-k8s.md#deploy-a-function-with-the-nuclio-cli).

