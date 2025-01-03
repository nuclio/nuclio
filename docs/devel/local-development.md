# Fast development cycles on Nuclio services

This guide will guide you through the process of setting up Nuclio services (dashboard and controller) locally for testing your changes and provide faster development cycles.

#### In This Document

- [Prerequisites](#prerequisites)

- [Running Nuclio services](#running-nuclio)

- [Running a function locally](#running-function)

<a id="prerequisites"></a>
## Prerequisites

Ensure that your setup includes the following prerequisite components:

- Linux or OSX
- Git
- Docker version 19.03+
- The Go toolchain (CI tests with 1.23, best use that)
- Kubernetes version 1.24+ (mostly for testing) - `minikube` recommended; (you can follow
  the [Minikube getting-started guide](../setup/minikube/getting-started-minikube.md))
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
   - In Goland - open the `dashboard-kube` run configuration. Make sure the program arguments are as follows:
     ```sh
     --platform kube --platform-config hack/env/platform_config.yaml --namespace default --registry localhost:5000 --run-registry localhost:5000 --templates-archive-address "" --templates-git-repository "https://github.com/nuclio/nuclio-templates.git"
     ```

      > **Note:** By default, when building a function image, the dashboard will try to pull the base image from the remote registry with the "latest" tag. 
      If you have the base image locally you can specify the env-var: `NUCLIO_DASHBOARD_NO_PULL_BASE_IMAGES=true`

   - If you want to run a specific Nuclio version, you can add the following flags to `Go tool aruments` in the run configuration:
      ```sh
      -ldflags="-X github.com/v3io/version-go.label=<Nuclio-version>"
      ```
      Or set the following env-var: `NUCLIO_LABEL=<Nuclio-version>`

   - Run it - see that it's listening on port `8070`
   

5. Run Controller:
   - In Goland - open the  `contorller-kube` run configuration. Make sure the program arguments are as follows, with your own kube/config directory:
   ```sh
   --platform-config hack/env/platform_config.yaml --namespace default --kubeconfig-path path/to/.kube/config
   ```
   - Run it.


6. Run UI:
   - Open `pkg/dashboard/ui` in a terminal
   - Run:
      ```sh
      npm install
      gulp watch --dev
      ```
      And make sure it's listening on port `8000`
      
   - Open `localhost:8000` in a browser and use Nuclio as you please! 
   

You can now perform operations on the Nuclio UI and view the dashboard and controller logs live on Goland's run console.

<a id="running-function"></a>
## Running a function locally

We can also run a function locally, and debug the processor code using Goland.

1. Create a function yaml file,  (This is essentially a function config, since the processor runs inside a single function.
   e.g for a Go function: hack/env/golang.yaml file:
   ```yaml
   meta:
      name: "my-func"
      namespace: "nuclio"
   spec:
      runtime: golang
      handler: nuclio:builtin
      readinessTimeoutSeconds: 60
      logger:
         level: debug
      replicas: 1
   ```
   
2. Open `cmd/processor/main.go` and run the `main` function with the following run configurations:]
   ```shell
   --config hack/env/golang.yaml --platform-config hack/env/platform_config.yaml
   ```
   
3. Run it / debug it. You can now debug the processor code, and see the function logs on the console.