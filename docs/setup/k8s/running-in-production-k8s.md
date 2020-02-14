# Running Nuclio over Kubernetes in production

After familiarizing yourself with Nuclio, and [deploying it over k8s](/docs/setup/k8s/getting-started-k8s.md) you may find yourself looking for extra configuration knobs, and proper practices for using it in a production environment.
In [Iguazio](https://www.iguazio.com/) we integrated Nuclio as part of our [Data science PaaS](https://www.iguazio.com/platform/), and it is in extensive use in production, for both our customers and ourselves, running various workloads.

Here you will find more advanced configuration options, and some practices which address the needs of running Nuclio in production environments. 


#### In this document

- [Preferred installation method](#preferred-installation-method)
- [Multi Tenancy](#multi-tenancy)
- [Dark site operation](#dark-site-operation)
- [Using Kaniko as an image builder](#using-kaniko-as-an-image-builder)

## Preferred installation method

Even though you may, of course, use any of the available methods, we find that heavy-lifting over k8s is best done with helm charts.
Nuclio's [helm chart](/hack/k8s/helm/nuclio/) is our preferred mode of deploying Nuclio in [Iguazio](https://www.iguazio.com/) as of writing this document.
This is also where you'll find most of the production oriented features appearing first, and is the most tightly maintained form of deployment.

Below is a quick example of how to setup the latest stable version of nuclio via helm charts:

- Create a namespace for your functions, and a secret with your intended registry's credentials
    ```sh
    kubectl create namespace nuclio
    ```
    ```sh
    read -s mypassword
    <enter your password>
    
    kubectl create secret docker-registry registry-credentials --namespace nuclio \
        --docker-username <username> \
        --docker-password $mypassword \
        --docker-server <URL> \
        --docker-email ignored@nuclio.io
    
    unset mypassword
    ```
 - Checkout the nuclio project and install nuclio from its helm chart: 
    ```sh
    git checkout https://github.com/nuclio/nuclio.git
    
    helm install \
        --set registry.secretName=registry-credentials \
        --set registry.pushPullUrl=<your registry URL> \
        --set controller.image.tag=latest-amd64 \
        --set dashboard.image.tag=latest-amd64\
        .
    ```
  
  See [the helm chart's values file](/hack/k8s/helm/nuclio/values.yaml) for a full list of configurable parameters

## Multi Tenancy

Implementation of multi-tenancy can be done in many different forms and to various degrees. Our experience have led us to adopt the k8s approach of tenant isolation using namespaces.
- To achieve tenant separation for various nuclio projects and functions, and to avoid cross-tenant contamination and resource races, we have opted to deploy in each namespace a fully functioning Nuclio deployment, and configure Nuclio's controller to be namespaced.
  This means the controller will handle Nuclio resources (functions, function-events, projects) only within its own namespace. This is supported via `Values.controller.namespace` in the helm chart values.  
- To provide ample separation on the docker image level, we highly recommend that the Nuclio deployments of various tenants will not share docker registries, or will not share a tenant, if a multi-tenant registry is used (like `docker.io` or `quay.io`) 
 
## Dark site operation

We have received various questions about running Nuclio in air-gapped environments. Nuclio is fully dark-site compatible, and supports the appropriate configuration to avoid any outside access.
These guidelines refer to more advanced use cases and they assume the surrounding work (devops!) can be done by the targeted user.
We know these things can get tricky! If you want access to a fully-managed, darksite-friendly, batteries-included, Nuclio deployment, packaged with **Lots** of other goodies - do check out the enterprise grade [Iguazio Data science PaaS](https://www.iguazio.com/platform/)! 

That being said, here are a few guidelines to get you on your way:

- Set `Values.offline=true` in the helm values, to put nuclio in "offline" mode. Set `dashboard.baseImagePullPolicy=Never`.
  In this mode you'll also want to freeze and control the values of all the `*.image.repository` and `*.image.tag` configuration keys to the image names and tags which are available to your k8s deployment in the offline environment.
  And `*.image.pullPolicy` to `Never` or `IfNotPresent` to make sure k8s won't try to access the web to fetch images.
- Needless to say, in this scenario, you will configure nuclio with `registry.pushPullUrl` which is reachable from your system.
- The processor and onbuild images will also have to be available to the dashboard in your environment, as they must be available for the building process - (by `docker build`, or [kaniko](#using-kaniko-as-an-image-builder)).
  This can be tricky as you have to either make those images available to the k8s docker daemon or pull-able from a reachable registry, where they should be preloaded. Use `Values.registy.defaultBaseRegistryURL` to point nuclio at searching in your registry for those images, rather then at the default location of `quay.io/nuclio`.
  To save some work on setting up a registry and preloading the onbuild images to it (or as a reference to what it should include) - take a look at the [prebaked-registry](https://github.com/nuclio/prebaked-registry).
- For the Nuclio templates library to be available to you, you'll have to package that yourself, and have it served locally, somewhere within reach of your system. To point Nuclio to it, set `Values.dashboard.templatesArchiveAddress` to where you serve the templates.


## Using Kaniko as an image builder

When dealing with production deployments, bind-mounting the docker socket into Nuclio dashboard's pod is a bit of a no-no. Having access to the host machine's docker daemon by the Nuclio dashboard is akin to giving it root access to the machine.
This is understandably a concern for real production use cases. Ideally, no pod should access the docker daemon directly, but since Nuclio is a docker based serverless framework, it needs the ability to build [OCI images](https://github.com/opencontainers/image-spec) at run time.
While there are several alternatives to bind-mounting the docker socket, we have opted to integrate [Kaniko](https://github.com/GoogleContainerTools/kaniko) as a production-ready alternative to build ICO images in a secured way in latest versions.
It is well maintained, stable, easy to use, and provides an ample set of features.
Kaniko is available to use as of version 1.3.15 of Nuclio, currently only on k8s.

To deploy nuclio and direct it to use the Kaniko engine to build images, apply the appropriate helm values as such:

    ```sh
    helm install \
        --set registry.secretName=registry-credentials \
        --set registry.pushPullUrl=<your registry URL> \
        --set dashboard.containerBuilderKind=kaniko \
        --set controller.image.tag=latest-amd64 \
        --set dashboard.image.tag=latest-amd64\
        .
    ```

Simple enough, right?

A few notes though:
- If running in an air-gapped / "dark" environment, kaniko's executor image must also be available to your k8s cluster
- Kaniko *requires* that you work with a registry, which is used to push the resulting function images to, it is no longer possible to have an image built and be available on the host docker daemon.
  This means you must configure a `Values.registry.pushPullUrl` for kaniko to push the resulting images to, as well as possibly `Values.registry.defaultBaseRegistryURL` if the onbuild / base images are not preloaded or otherwise available on your `pushPullUrl` at the time of function build.

We should also mention that we are looking into enabling docker-in-docker (dind) as a possible mode of operation.


