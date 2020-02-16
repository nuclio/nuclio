# Running Nuclio over Kubernetes in production

After familiarizing yourself with Nuclio, and [deploying it over k8s](/docs/setup/k8s/getting-started-k8s.md) you may find yourself looking for extra configuration knobs, and proper practices for using it in a production environment.
In [Iguazio](https://www.iguazio.com/) we integrated Nuclio as part of our [Data science PaaS](https://www.iguazio.com/platform/), and it is in extensive use in production, for both our customers and ourselves, running various workloads.

Here you will find more advanced configuration options, and some practices which address the needs of running Nuclio in production environments. 


#### In this document

- [Preferred installation method](#preferred-installation-method)
- [Version freezing](#version-freezing)
- [Multi Tenancy](#multi-tenancy)
- [Air gapped (dark site) operation](#air-gapped-dark-site-operation)
- [Using Kaniko as an image builder](#using-kaniko-as-an-image-builder)


## Preferred installation method

Even though you may, of course, use any of the available methods, we find that heavy-lifting over k8s is best done with helm charts.
Nuclio's [helm chart](/hack/k8s/helm/nuclio/) is our preferred mode of deploying Nuclio in [Iguazio](https://www.iguazio.com/) as of writing this document.
This is also where you'll find most of the production oriented features appearing first, and is the most tightly maintained form of deployment.

Below is a quick example of how to setup the a specific stable version of nuclio (1.3.14) via helm charts:

- Create a namespace for your functions, and a secret with your intended registry's credentials

    ```sh
    kubectl create namespace nuclio
    ```

    ```sh
    read -s mypassword
    <enter your password>
    
    kubectl create secret docker-registry registry-credentials \
        --docker-username <username> \
        --docker-password $mypassword \
        --docker-server <URL> \
        --docker-email <some email>
    
    unset mypassword
    ```

- Copy the secret to the nuclio namespace, since k8s does not allow for secret sharing between namespaces:
    ```sh
    kubectl get secret registry-credentials -n default -o yaml \
    | sed s/"namespace: default"/"namespace: nuclio"/ \
    | kubectl apply -f -
    ```

 - Checkout the nuclio project and install nuclio from its helm chart: 

    ```sh
    git checkout https://github.com/nuclio/nuclio.git
    
    helm install \
        --set registry.secretName=registry-credentials \
        --set registry.pushPullUrl=<your registry URL> \
        --set controller.image.tag=<version>-amd64 \
        --set dashboard.image.tag=<version>-amd64 \
        ./hack/k8s/helm/nuclio/
    ```

  See [the helm chart's values file](/hack/k8s/helm/nuclio/values.yaml) for a full list of configurable parameters


## Multi Tenancy

Implementation of multi-tenancy can be done in many different forms and to various degrees. Our experience have led us to adopt the k8s approach of tenant isolation using namespaces.
- To achieve tenant separation for various nuclio projects and functions, and to avoid cross-tenant contamination and resource races, we have opted to deploy in each namespace a fully functioning Nuclio deployment, and configure Nuclio's controller to be namespaced.
  This means the controller will handle Nuclio resources (functions, function-events, projects) only within its own namespace. This is supported via `Values.controller.namespace` in the helm chart values.  
- To provide ample separation on the docker image level, we highly recommend that the Nuclio deployments of various tenants will not share docker registries, or will not share a tenant, if a multi-tenant registry is used (like `docker.io` or `quay.io`) 


## Version freezing

- Working in production, you need reproducibility and consistency. This means you will not be working with latest stable version, but qualify a specific version and freeze it in your Nuclio configuration ([helm chart values file](/hack/k8s/helm/nuclio/values.yaml)).
  Stick with this version until you qualify a newer one to work with your system. Since we adhere to backwards compatibility standards between patch versions, and even minor version bumps usually do not break major functionality, the process of qualifying a newer nuclio version should hopefully be short and easy.
  To version freeze via helm values, set all of the `*.image.repository` and `*.image.tag` configuration keys to the image names and tags which represenr your chosen version's images and are available to your k8s deployment (in case of an [air-gapped installation](#air-gapped-dark-site-operation)).
 
 
## Air gapped (dark site) operation

We have received various questions about running Nuclio in air gapped environments. Nuclio is fully air-gap compatible, and supports the appropriate configuration to avoid any outside access.
These guidelines refer to more advanced use cases and they assume the surrounding work (read: devops) can be done by the targeted user.
We know these things can get a bit tricky. If you want access to a fully-managed, air-gap-friendly, batteries-included, Nuclio deployment, packaged with **Lots** of other goodies - do check out the enterprise grade [Iguazio Data science PaaS](https://www.iguazio.com/platform/)! 

That being said, here are a few guidelines to get you on your way:

- Most definitely [version freeze](#version-freezing). Also, set `*.image.pullPolicy` to `Never` or `IfNotPresent` to make sure k8s won't try to access the web to fetch images at any point in time.
- Set `Values.offline=true` in the helm values, to put nuclio in "offline" mode. Set `dashboard.baseImagePullPolicy=Never`.
- Needless to say, in this scenario, you will have to configure nuclio with `registry.pushPullUrl` which is reachable from your system.
- The processor and onbuild images will also have to be accessible to the dashboard in your environment, as they are required for the building process - (by `docker build`, or [kaniko](#using-kaniko-as-an-image-builder)).
  This can be tricky as you have to either make those images available to the k8s docker daemon or pull-able from a reachable registry, where they should be preloaded. Use `Values.registy.defaultBaseRegistryURL` to point nuclio at searching in your registry for those images, rather then at the default location of `quay.io/nuclio`.
  To save some work on setting up a registry and preloading the onbuild images to it (or as a reference to what it should include) - take a look at the [prebaked-registry](https://github.com/nuclio/prebaked-registry).
- For the Nuclio templates library to be available to you, you'll have to package that yourself, and have it served locally, somewhere within reach of your system. To point Nuclio to it, set `Values.dashboard.templatesArchiveAddress` to where you serve the templates.


## Using Kaniko as an image builder

When dealing with production deployments, bind-mounting the docker socket into Nuclio dashboard's pod is a bit of a no-no. Having access to the host machine's docker daemon by the Nuclio dashboard is akin to giving it root access to the machine.
This is understandably a concern for real production use cases. Ideally, no pod should access the docker daemon directly, but since Nuclio is a docker based serverless framework, it needs the ability to build [OCI images](https://github.com/opencontainers/image-spec) at run time.
While there are several alternatives to bind-mounting the docker socket, we have opted to integrate [Kaniko](https://github.com/GoogleContainerTools/kaniko) as a production-ready alternative to build OCI images in a secured way in latest versions.
It is well maintained, stable, easy to use, and provides an ample set of features.
Kaniko is available to use as of version 1.3.15 of Nuclio, currently only on k8s.

To deploy nuclio and direct it to use the Kaniko engine to build images, apply the appropriate helm values as such:

```sh
helm install \
    --set registry.secretName=<your secret name> \
    --set registry.pushPullUrl=<your registry URL> \
    --set dashboard.containerBuilderKind=kaniko \
    --set controller.image.tag=<version>-amd64 \
    --set dashboard.image.tag=<version>-amd64\
    .
```

Simple enough, right?

A few notes though:
- If running in an air-gapped environment, kaniko's executor image must also be available to your k8s cluster
- Kaniko *requires* that you work with a registry, which is used to push the resulting function images to, it is no longer possible to have an image built and be available on the host docker daemon.
  This means you must configure a `Values.registry.pushPullUrl` for kaniko to push the resulting images to, as well as possibly `Values.registry.defaultBaseRegistryURL` if you operate in an air gapped environment.
- `quay.io` does not support nested repositories. If you are using kaniko as a container-builder, and `quay.io` as a registry (`--set registry.pushPullUrl=quay.io/<repo name>`), add the following to allow kaniko caching to succeed pushing:
    ```sh
    --set dashboard.kaniko.cacheRepo=quay.io/<repo name>/cache
    ```


We should also mention that we are looking into enabling docker-in-docker (dind) as a possible mode of operation.


