# Fast development cycles on Nuclio services

This guide will guide you through the process of setting up Nuclio services (dashboard and controller) locally for testing your changes and provide faster development cycles.

#### In This Document

- [Prerequisites](#prerequisites)

- [Running Nuclio services](#running-nuclio)

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

The guide assumes you're running Nuclio in kubernetes. If you're running it as a docker container, you can skip some steps.

<a id="running-nuclio"></a>
## Running Nuclio services

1. (Kubernetes only) Install Nuclio CRDs. You can install them by running `test/k8s/ci_assets/install_nuclio_crds.sh`.


2. In `hack/env` directory, create `platform_config.yaml` with the following body (you can update the example file in `hack/env/platform_config.yaml.example`):
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

# Kubernetes only
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
   1. In Goland - open the `dashboard-kube` run configuration. Make sure the program arguments are as follows:
   ```sh
   --platform kube --platform-config hack/env/platform_config.yaml --namespace default --registry localhost:5000 --run-registry localhost:5000
   ```
   
   2. If you want to run a specific Nuclio version, you can add the following flags to `Go tool aruments` in the run configuration:
   ```sh
   -ldflags="-X github.com/v3io/version-go.label=<Nuclio-version>"
   ```
   
   3. Run it - see that it's listening on port `8070`
   

5. Run Controller:
   1. In Goland - open the  `contorller-kube` run configuration. Make sure the program arguments are as follows, with your own kube/config directory:
   ```sh
   --platform-config hack/env/platform_config.yaml --namespace default --kubeconfig-path path/to/.kube/config
   ```
   2. Run it.
   

6. Run UI:
   1. Open `pkg/dashboard/ui` in a terminal
   2. Run:
   ```sh
   npm install
   gulp --dev
   ```
   And make sure it's listening on port `8000`
   
   3. Open `localhost:8000` in a browser and use Nuclio as you please! 
   

You can now perform operations on the Nuclio UI and view the dashboard and controller logs live on Goland's run console.
