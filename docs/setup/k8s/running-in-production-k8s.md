# Running Nuclio Over Kubernetes in Production

After familiarizing yourself with Nuclio and [deploying it over Kubernetes](/docs/setup/k8s/getting-started-k8s.md), you might find yourself in need of more information pertaining to running Nuclio in production.
Nuclio is integrated, for example, within the [Iguazio Data Science Platform](https://www.iguazio.com), which is used extensively in production, both by Iguazio and its customers, running various workloads.
This document describes advanced configuration options and best-practice guidelines for using Nuclio in a production environment.

#### In this document

- [The preferred deployment method](#the-preferred-deployment-method)
- [Freezing a qualified version](#freezing-a-qualified-version)
- [Multi-Tenancy](#multi-tenancy)
- [Air-gapped deployment](#air-gapped-deployment)
- [Using Kaniko as an image builder](#using-kaniko-as-an-image-builder)

<a id="the-preferred-deployment-method"></a>
## The preferred deployment method

There are several alternatives to deploying (installing) Nuclio in production, but the recommended method is by using [Helm charts](/hack/k8s/helm/nuclio/).
This is currently the preferred deployment method at Iguazio as it's the most tightly maintained, it's best suited for "heavy lifting" over Kubernetes, and it's often used to roll out new production-oriented features.

Following is a quick example of how to use Helm charts to set up a specific stable version of Nuclio.

1. Create a namespace for your Nuclio functions:

    ```sh
    kubectl create namespace nuclio
    ```

2. Create a secret with valid credentials for logging into your target container (Docker) registry:

    ```sh
    read -s mypassword
    <enter your password>

    kubectl --namespace nuclio create secret docker-registry registry-credentials \
        --docker-username <username> \
        --docker-password $mypassword \
        --docker-server <URL> \
        --docker-email <some email>

    unset mypassword
    ```

3. Add and Install `nuclio` Helm chart:

    ```sh
    helm repo add nuclio https://nuclio.github.io/nuclio/charts
    helm install nuclio \
        --set registry.secretName=registry-credentials \
        --set registry.pushPullUrl=<your registry URL> \
        nuclio/nuclio
    ```

> NOTE: for a full list of configuration parameters, see the Helm values file ([**values.yaml**](/hack/k8s/helm/nuclio/values.yaml))

<a id="multi-tenancy"></a>
## Multi-Tenancy

Implementation of multi-tenancy can be done in many ways and to various degrees.
The experience of the Nuclio team has lead to the adoption of the Kubernetes approach of tenant isolation using namespaces.
Note:

- To achieve tenant separation for various Nuclio projects and functions, and to avoid cross-tenant contamination and resource races, a fully functioning Nuclio deployment is used in each namespace and the Nuclio controller is configured to be namespaced.
  This means that the controller handles Nuclio resources (functions, function events, and projects) only within its own namespace.
  This is supported by using the `controller.namespace` and `rbac.crdAccessMode` [Helm values](/hack/k8s/helm/nuclio/values.yaml) configurations.
- To provide ample separation at the level of the container registry, it's highly recommended that the Nuclio deployments of multiple tenants either don't share container registries, or that they don't share a tenant when using a multi-tenant registry (such as `registry.hub.docker.com` or `quay.io`).

<a id="freezing-a-qualified-version"></a>
## Freezing a qualified version

When working in production, you need reproducibility and consistency.
It's therefore recommended that you don't use the latest stable version, but rather qualify a specific Nuclio version and "freeze" it in your configuration.
Stick with this version until you qualify a newer version for your system.
Because Nuclio adheres to backwards-compatibility standards between patch versions, and even minor version updates don't typically break major functionality, the process of qualifying a newer Nuclio version should generally be short and easy.

To use Helm to freeze a specific Nuclio version, set all of the `*.image.repository` and `*.image.tag` [Helm values](/hack/k8s/helm/nuclio/values.yaml) to the names and tags that represent the images for your chosen version.
Note the configured images must be accessible to your Kubernetes deployment (which is especially relevant for [air-gapped deployments](#air-gapped-deployment)).

<a id="air-gapped-deployment"></a>
## Air-gapped deployment

Nuclio is fully compatible with execution in air-gapped environments ("dark sites"), and supports the appropriate configuration to avoid any outside access.
The following guidelines refer to more advanced use cases and are based on the assumption that you can handle the related DevOps tasks.
Note that such implementations can get a bit tricky; to access a fully-managed, air-gap friendly, "batteries-included", Nuclio deployment, which also offers plenty of other tools and features, check out the enterprise-grade [Iguazio Data Science Platform](https://www.iguazio.com/platform/).
If you select to handle the implementation yourself, follow these guidelines; the referenced configurations are all [Helm values](/hack/k8s/helm/nuclio/values.yaml):

- Set `*.image.repository` and `*.image.tag` to [freeze a qualified version](#version-freezing), and ensure that the configured images are accessible to the Kubernetes deployment.
- Set `*.image.pullPolicy` to `Never` or to `IfNotPresent` to ensure that Kubernetes doesn't try to fetch the images from the web.
- Set `offline` to `true` to put Nuclio in "offline" mode.
- Set `dashboard.baseImagePullPolicy` to `Never`.
- Set `registry.pushPullUrl` to a registry URL that's reachable from your system.
- <a id="air-gapped-envir-base-n-onbuild-images"></a>Ensure that base, "onbuild", and processor images are accessible to the dashboard in your environment, as they're required for the build process (either by `docker build` or [Kaniko](#using-kaniko-as-an-image-builder)).
  You can achieve this using either of the following methods:

  - Make the images available on the host Docker daemon (local cache).
  - Preload the images to a registry that's accessible to your system, to allow pulling the images from the registry.
    When using this method, set `registy.dependantImageRegistryURL` to the URL of an accessible local registry that contains the preloaded images (thus overriding the default location of `quay.io/nuclio`, which isn't accessible in air-gapped environments).
    <br/><br/>
    > **Note:** To save yourself some work, you can use the [prebaked Nuclio registry](https://github.com/nuclio/prebaked-registry), either as-is or as a reference for creating your own local registry with preloaded images.

- To use the Nuclio templates library (optional), package the templates into an archive; serve the templates archive via a local server whose address is accessible to your system; and set `dashboard.templatesArchiveAddress` to the address of this local server.

<a id="using-kaniko-as-an-image-builder"></a>
## Using Kaniko as an image builder

When dealing with production deployments, you should avoid bind-mounting the Docker socket to the service pod of the Nuclio dashboard; doing so would allow the dashboard access to the host machine's Docker daemon, which is akin to giving it root access to your machine.
This is understandably a concern for real production use cases.
Ideally, no pod should access the Docker daemon directly, but because Nuclio is a container-based serverless framework, it needs the ability to build [OCI images](https://github.com/opencontainers/image-spec) at run time.
While there are several alternatives to bind-mounting the Docker socket, the selected Nuclio solution, starting with Nuclio version 1.3.15, is to integrate [Kaniko](https://github.com/GoogleContainerTools/kaniko) as a production-ready method of building OCI images in a secured way.
Kaniko is well maintained, stable, easy to use, and provides an extensive set of features.
Nuclio currently supports Kaniko only on Kubernetes.

To deploy Nuclio and direct it to use the Kaniko engine to build images, use the following [Helm values](/hack/k8s/helm/nuclio/values.yaml) parameters; replace the `<...>` placeholders with your specific values:

```sh
helm upgrade --install --reuse-values nuclio \
    --set registry.secretName=<your secret name> \
    --set registry.pushPullUrl=<your registry URL> \
    --set dashboard.containerBuilderKind=kaniko \
    --set controller.image.tag=<version>-amd64 \
    --set dashboard.image.tag=<version>-amd64\
    nuclio/nuclio
```

This is rather straightforward; however, note the following:

- When running in an [air-gapped environment](#air-gapped-deployment), Kaniko's executor image must also be available to your Kubernetes cluster.
- Kaniko requires that you work with a registry to which push the resulting function images.
  It doesn't support accessing images on the host Docker daemon.
  Therefore, you must set `registry.pushPullUrl` to the URL of the registry to which Kaniko should push the resulting images, and in air-gapped environments, you must also set `registry.defaultBaseRegistryURL` and `registry.defaultOnbuildRegistryURL` to the URL of an accessible local registry that contains the preloaded base, "onbuild", and processor images (see [Air-gapped deployment](#air-gapped-envir-base-n-onbuild-images)).
- `quay.io` doesn't support nested repositories.
  If you're using Kaniko as a container builder and `quay.io` as a registry (`--set registry.pushPullUrl=quay.io/<repo name>`), add the following to your configuration to allow Kaniko caching to push successfully; (replace the `<repo name>` placeholder with the name of your repository):
    ```sh
    --set dashboard.kaniko.cacheRepo=quay.io/<repo name>/cache
    ```

> **Note:** The Nuclio team is also looking into enabling Docker-in-Docker (DinD) as a possible mode of operation.

