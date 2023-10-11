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

#### Invoking a function using nuctl on Kubernetes

While in most cases using `nuctl` on the [Local Docker](#docker) platform enables you to invoke your function by running the
`nuctl invoke` command on the host, in Kubernetes platform this is a bit trickier - and will most likely not work
by default for some configurations. This means, for example, that `nuctl invoke` command will not be able to work
unless your function is explicitly exposed or reachable from wherever you run `nuctl invoke`.

See [exposing a function](/docs/tasks/deploying-functions.md#exposing-a-function) for more details.

For your convenience, when deploying a function using `nuctl`, exposing it via a `NodePort` can be easily done by using the
CLI arg `--http-trigger-service-type=nodePort`.


### Redeploying functions

Currently `redeploy` command is the only command which uses dashboard API. Namely, it uses `Patch` request.  
Main difference of `redeploy` command from `deploy` command is that during redeployment we do not build image, which 
means that image should be presented in registry. 

Use-cases:
* to [redeploy imported functions](#redeploying-imported-functions) (for instance, after upgrade)
* simply redeploy already existing function

Functions can be successfully redeployed to either `ready` (default) or `scaledToZero` states. 
To be able to redeploy function as `scaledToZero`, minimum replicas of function should be equal to zero.

Command flags:
* `verify-external-registry` - functions will be redeployed only if registry is external
* `save-report` - allows to save redeployment report to a file. Path can be specified via `report-file-path` argument
* `report-file-path` - report file path. Default is `./nuctl-redeployment-report.json`
* `from-report` - redeploy failed retryable functions from report
* `imported-only` - redeploy only imported functions
* `desired-state` - redeploy to specific status. Choose any of those statuses: `[scaledToZero, ready]`.
* `exclude-projects` - exclude projects
* `exclude-functions` - exclude functions
* `exclude-functions-with-gpu` - exclude only functions with GPU
* `wait` - waits for redeployment to be completed
* `wait-timeout` - waits timeout duration for the function redeployment

#### Redeployment report interpretation
Redeployment report contains 3 lists with function names - `Success, Skipped, Failed`.
`Success` list contains names of functions which were successfully redeployed.
`Skipped` list contains functions redeployment process for which was skipped.
`Failed` list contains function which weren't successfully redeployed and for each failed function report contains fail 
descriptions and boolean variable which shows if command rerun can help.

If `retryable == false` in your report there are possible reasons:

* No image field found in function config spec (check `spec.image`)
* Desired state is `scaledToZero` but `minReplicas > 0`
* `imported-flag` was used in redeploy command and function not in `imported` state

#### Redeploying imported functions

To redeploy only imported functions use `--imported-only` flag.

Imported function can be redeployed to the state it had before being imported. To be able to do so, function should
be exported with previous state which means that function will have a `nuclio.io/previous-state` annotation.