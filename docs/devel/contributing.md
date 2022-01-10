# How to Contribute to Nuclio

This guide will guide you through the process of setting up a development environment and contributing to Nuclio.

#### In This Document

- [Prerequisites](#prerequisites)
- [Getting the source code](#get-source)
- [Setting up a GoLand project](#goland-setup)
- [Submitting a PR](#submitting-a-pr)

<a id="prerequisites"></a>
## Prerequisites

Ensure that your setup includes the following prerequisite components:

- Linux or OSX
- Git
- Docker version 19.03+
- The Go toolchain (CI tests with 1.17, best use that)
- Kubernetes version 1.20+ (mostly for testing) - `minikube` recommended; (you can follow
  the [Minikube getting-started guide](/docs/setup/minikube/getting-started-minikube.md))

<a id="get-source"></a>
## Getting the source code

Fork the Nuclio GitHub repository and clone it:

```sh
git clone https://github.com/<your username>/nuclio.git ~/nuclio
```

Check out the `development` branch, because that's where all the goodness happens:

```sh
cd ~/nuclio && git checkout development
```

Now, use `make modules` to get Nuclio's dependencies and modules

Build Nuclio artifacts (`nuctl`, container images):

```sh
make build
```

You should now have quite a few `nuclio/<something>` images tagged as `latest-amd64`, along
with `nuctl-latest-<os>-amd64` with a `nuctl` symbolic link under `$GOPATH/bin`.

> Note: to make nuclio container images available for arm64 architecture run `NUCLIO_ARCH=arm64 make build`

Now, run a few unit tests:

```sh
make lint test-unit
```

This is a short test suite, and requires only Docker.

Note: Writing (Go) Nuclio tests requiring it having build tag annotation at the first line Use the following build tag
annotations to match the test purpose:

- test_unit - unit testings
- test_integration - integration testings
- test_kube - requires kubernetes platform
- test_local - requires docker platform
- test_iguazio - requires Iguazio system
- test_broken - test suite is broken, cannot be executed on CI

To run Kubernetes `nuctl` suite locally:

`NUCTL_REGISTRY=<registry> make test-k8s-nuctl`

To run Docker `nuctl` suite locally:

`make test-docker-nuctl`

To run Kubernetes platform integration test locally:

```sh

# Installing nuclio CRDs
sh test/k8s/ci_assets/install_nuclio_crds.sh

# Running local Docker registry
docker run --rm -d -p 5000:5000 registry:2

make test-k8s
```

To run integration tests for docker platform, run:

```sh
make test
```

> Note: it may take a while ~90 minutes

On Nuclio CI, we run nuctl test suites against both Docker and Kubernetes platforms.

Running more comprehensive end-to-end tests on kubernetes is currently done manually.

When you're done, create a feature branch from the `development` branch; (Nuclio follows the GitFlow branching model):

```sh
git checkout -b my-feature
```

<a id="goland-setup"></a>
## Setting up a GoLand project

The Nuclio team is a fan of GoLand and uses it heavily for Go projects. It was decided not to include the **.idea**
files at this time, but it's very easy to create run/debug targets and use the debugger

1. Clone nuclio `git clone git@github.com:nuclio/nuclio.git > ~/nuclio`
2. Open GoLand and **File > Open** and select `~/nuclio`
3. Enable go modules **GoLand > Preferences > Go > Go Modules (vgo)** and ensure `Enable Go Modules` box is checked
4. Run configurations are (currently) available for tests only


For developing in nuclio locally, see [Local development](/docs/devel/local-development.md).

<a id="goland-versioning-note"></a>
### Versioning note

All Nuclio artifacts are versioned. They take their versions from variables in **v3io/version-go/version.go**.

During link time, the linker set the version variables using the `-X` flag.

Since there is an auto-fallback to "latest", if you want to use versioned binaries, make sure to pass the following as
part of the `Go tool arguments` in the Run/Debug configuration:

```
-ldflags="-X github.com/v3io/version-go.label=1.7.2"
``` 

<a id="goland-run-go-processor"></a>
### Running the processor (Go)

Under normal circumstances, the function provided by the user is compiled as a Go plugin that's loaded by the processor.
If you need to debug this plugin loading mechanism, you're advanced enough to find your way around. If all you want to
do is test a new feature on the processor, the easiest way to achieve this is to use Go with the built-in
handler (`nuclio:builtin`). When you specify `handler: nuclio:builtin` in the processor configuration file, the Go
runtime doesn't try to load a plugin and simply uses `pkg/processor/runtime/golang/runtime.go:builtInHandler()`. Feel
free to modify that function, just don't check in your changes.

The processor configuration file is basically the content of your **function.yaml** function-configuration file:

```yaml
spec:
  runtime: golang
  handler: nuclio:builtin
  logger:
    level: debug
  triggers: { }
```

Another configuration that should be given to the processor is the platform configuration, which looks like this:

```yaml
logger:
  sinks:
    myStdoutLoggerSink:
      kind: stdout
  system:
    - level: debug
      sink: myStdoutLoggerSink
  functions:
    - level: debug
      sink: myStdoutLoggerSink
```

Create these two configuration files in your preferred location, and pass `--config <path to processor.yaml>
--platform-config <path to platform-config.yaml` as `Program arguments` in the Run/Debug configuration.

For more information about the platform configuration,
see [Configuring a Platform](/docs/tasks/configuring-a-platform.md#configuration-elements). For information about the
function configuration, see
the [Function-Configuration Reference](/docs/reference/function-configuration/function-configuration-reference.md).

<a id="goland-run-cli"></a>
### Running the Nuclio CLI (nuctl)

There's nothing special required to run `nuctl`, but you may want to pass `--platform local` in case you don't want to
work with Kubernetes.

<a id="submitting-a-pr"></a>
## Submitting a PR

Your PRs will go through Github Action CI and code review. Make sure to follow
the [coding conventions](/docs/devel/coding-conventions.md) and run `make fmt lint` before submitting a PR, that would
save you some time failing on CI linting and test validations

