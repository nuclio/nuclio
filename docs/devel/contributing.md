# How to contribute to nuclio
This guide will guide you through the process of setting up a development environment and contributing to nuclio. 

## Set up some prerequisites
Obviously, you'll need:
- Linux or OSX
- git
- Docker (version 17.05+, since `nuclio` uses [multi-stage builds](https://docs.docker.com/engine/userguide/eng-image/multistage-build/))
- The Go toolchain (CI tests with 1.9, best use that)
- A `GOPATH` directory and `GOPATH` environment variable set to that
- Kubernetes 1.7+ (for testing, mostly) - `minikube` recommended (you can follow the [minikube getting started guide](/docs/setup/k8s/install/k8s-install-minikube.md))

## Getting the source
Fork nuclio / nuclio-sdk and clone them locally:

```bash
git clone https://github.com/<your username>/nuclio.git $GOPATH/src/github.com/nuclio/nuclio
git clone https://github.com/<your username>/nuclio-sdk.git $GOPATH/src/github.com/nuclio/nuclio-sdk
```

Build nuclio artifacts (nuctl, docker images):

```bash
cd $GOPATH/src/github.com/nuclio/nuclio && unset NUCLIO_TAG && make build
```

You should now have quite a few `nuclio/<something>` images tagged as `latest-amd64` along with `nuctl-latest-<os>-amd64`, symlinked as `nuctl` under `$GOPATH/bin`. Let's run a few tests:

```bash
make lint test
``` 

This may take a while (about 5 minutes) and only needs docker. End to end testing on Kubernetes is still done manually.

Create a feature branch from development (nuclio follows gitflow):

```bash
git checkout -b my-feature
```

## Setting up a GoLand project
We <3 GoLand and use it heavily for Go projects. We chose not to include the `.idea` files at this time, but it is super easy to create run/debug targets and use the debugger:
1. Create a new project pointing to $GOPATH/src/github.com/nuclio/nuclio
2. Click "GoLand-EAP" -> Preferences -> Go -> GOPATH and add the value of $GOPATH   

### A note about versioning
All artifacts in `nuclio` are versioned. They take their versions from one of two sources (in the following order):
1. Variables in `pkg/version/version.go`, set by the linker during link time using `-X`
2. A version file, residing at `/etc/nuclio/version_info.json`

As of now, there is no auto-fallback to "latest" to prevent un-versioned binaries ever getting created. As such, make sure to pass the following to `Go tool arguments` in the Run/Debug configuration:
```
-i -ldflags="-X github.com/nuclio/nuclio/pkg/version.label=latest -X github.com/nuclio/nuclio/pkg/version.arch=amd64"
``` 

### Running the processor (Go)
Under normal circumstances, the function provided by the user will be compiled as a Go plugin loaded by the processor. If you need to debug this plugin loading mechanism, you're advanced enough to find your way around. If all you want to do is test a new feature on the processor, the easiest way to go about this is to use Go with the "built in" handler. 

By specifying `handler: nuclio:builtin` in the processor configuration file, the Go runtime will not try to load a plugin and simply use `pkg/processor/runtime/golang/runtime.go:builtInHandler()`. Feel free to modify that function, just don't check it in. 

The processor configuration file currently has this schema (in the future it will be changed to the schema of `function.yaml`):

```yaml
dataBindings: {}
function:
  handler: nuclio:builtin
  runtime: golang
logger:
  level: debug
triggers: {}
```

Create this file somewhere and pass `--config <path to processor.yaml>` as `Program arguments` in the Run/Debug configuration.

### Running nuctl
There's nothing special required to run `nuctl`, but you may want to pass `--platform local` in case you don't want to work with Kubernetes. 

## Submitting a PR

Your PRs will go through travis CI and code review. Make sure to follow the [coding conventions](/docs/devel/coding-conventions.md) and run `make lint test` before submitting to make sure you haven't broken anything.
