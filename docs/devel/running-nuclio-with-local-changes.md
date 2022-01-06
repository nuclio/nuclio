# Running Nuclio with local changes

This guide will guide you through the process of setting up Nuclio locally for testing your changes and provide faster development cycles.

#### In This Document

- [Prerequisites](#prerequisites)

- [Running Nuclio](#running-nuclio)

<a id="prerequisites"></a>
## Prerequisites

Ensure that your setup includes the following prerequisite components:

- Linux or OSX
- Git
- Docker version 19.03+
- The Go toolchain (CI tests with 1.17, best use that)
- Kubernetes version 1.20+ (mostly for testing) - `minikube` recommended; (you can follow
  the [Minikube getting-started guide](/docs/setup/minikube/getting-started-minikube.md))
- Node version 10.x
- Goland IDE

<a id="running-nuclio"></a>
## Running Nuclio

1. Install Nuclio CRDs. You can install them by running `test/k8s/ci_assets/install_nuclio_crds.sh`.


2. Create a configs folder outside of the Nuclio repo. Inside it, create `configs/platform_config.yaml` with the following body:
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

kube:
  kubeConfigPath: <your-home-dir>/.kube/config

containerBuilderConfiguration:
  DefaultOnbuildRegistryURL: "localhost:5000"
```

3. Run local registry:
```sh
docker run --rm -d -p 5000:5000 registry:2
```
4. Run Dashboard:
   1. Open `cmd/dashboard/main.go`
   2. In Goland - create a run configuration for the `main` function with the following program arguments:
   ```sh
   --platform kube --platform-config path/to/configs/platform_config.yaml --namespace default --registry localhost:5000 --run-registry localhost:5000
   ```
   3. Run it - see that it's listening on port `8070`
   

5. Run Controller:
   1. Open `cmd/controller/main.go`
   2. In Goland - create a run configuration for the `main` function with the following program arguments:
   ```sh
   --platform-config path/to/configs/platform_config.yaml --namespace default --kubeconfig-path path/to/.kube/config
   ```
   3. Run it.
   

6. Run UI:
   1. Open `pkg/dashboard/ui` in a terminal
   2. Run:
   ```sh
   npm install
   gulp --dev
   ```
   3. Open `localhost:8000` and use Nuclio! 
   