# Running Nuclio Over Kubernetes in Production

After familiarizing yourself with Nuclio and [deploying it over Kubernetes](../../setup/k8s/getting-started-k8s.md), you might find yourself in need of more information pertaining to running Nuclio in production.
Nuclio is integrated, for example, within the [Iguazio Data Science Platform](https://www.iguazio.com), which is used extensively in production, both by Iguazio and its customers, running various workloads.
This document describes advanced configuration options and best-practice guidelines for using Nuclio in a production environment.

## In This Document

- [The preferred deployment method](#the-preferred-deployment-method)
- [Freezing a qualified version](#freezing-a-qualified-version)
- [Multi-Tenancy](#multi-tenancy)
- [Securing the Dashboard](#securing-the-dashboard)
- [Air-gapped deployment](#air-gapped-deployment)
- [Using Kaniko as an image builder](#using-kaniko-as-an-image-builder)

<a id="the-preferred-deployment-method"></a>
## The preferred deployment method

There are several alternatives to deploying (installing) Nuclio in production, but the recommended method is by using [Helm charts](https://github.com/nuclio/nuclio/tree/development/hack/k8s/helm/nuclio/).
This is currently the preferred deployment method at Iguazio as it's the most tightly maintained, it's best suited for "heavy lifting" over Kubernetes, and it's often used to roll out new production-oriented features.

Following is a quick example of how to use Helm charts to set up a specific stable version of Nuclio.

1. Create a namespace for your Nuclio functions:

    ```sh
    kubectl create namespace nuclio
    ```

2. Create a secret with valid credentials for logging into your target container (Docker) registry:

    ```sh
    read -s mypassword # <enter your password>

    kubectl --namespace nuclio create secret docker-registry registry-credentials \
        --docker-username <username> \
        --docker-password $mypassword \
        --docker-server <URL> \
        --docker-email <some email>

    unset mypassword
    ```
   > **Note:** If you are using Amazon's ECR see [using Kaniko with ECR](#Using Kaniko with amazon elastic container registry (ECR)) section.

3. Add the `nuclio` Helm chart:

    ```sh
    helm repo add nuclio https://nuclio.github.io/nuclio/charts
    ```
   Then install it:

    ```sh
    helm install nuclio \
        --set registry.secretName=registry-credentials \
        --set registry.pushPullUrl=<your registry URL> \
        --namespace nuclio \
        nuclio/nuclio
    ```

> **Note:** For a full list of configuration parameters, see the Helm values file ([**values.yaml**](https://github.com/nuclio/nuclio/tree/development/hack/k8s/helm/nuclio/values.yaml))

<a id="multi-tenancy"></a>
## Multi-Tenancy

Implementation of multi-tenancy can be done in many ways and to various degrees.
The experience of the Nuclio team has lead to the adoption of the Kubernetes approach of tenant isolation using namespaces.
Note:

- To achieve tenant separation for various Nuclio projects and functions, and to avoid cross-tenant contamination and resource races, a fully functioning Nuclio deployment is used in each namespace and the Nuclio controller is configured to be namespaced.
  This means that the controller handles Nuclio resources (functions, function events, and projects) only within its own namespace.
  This is supported by using the `controller.namespace` and `rbac.crdAccessMode` [Helm values](https://github.com/nuclio/nuclio/tree/development/hack/k8s/helm/nuclio/values.yaml) configurations.
- To provide ample separation at the level of the container registry, it's highly recommended that the Nuclio deployments of multiple tenants either don't share container registries, or that they don't share a tenant when using a multi-tenant registry (such as `registry.hub.docker.com` or `quay.io`).

<a id="securing-the-dashboard"></a>
## Securing the Dashboard

> **Security note:** The Nuclio Dashboard exposes an HTTP API (`/api/...`) that reads and writes function configuration, including any credentials carried by event triggers — for example the top-level `password` and `secret` fields, and nested values inside `attributes` such as `attributes.sasl.password` and `attributes.accesskey` on Kafka triggers or `attributes.password` on `v3io-stream` triggers.
> The default Helm deployment does **not** authenticate this API and does **not** mask trigger credentials at rest. Treat the Dashboard as a privileged control plane and harden it before exposing it on any network that is not fully trusted.

The default chart values are tuned for a quick development setup and assume the Dashboard sits on a trusted network. Before running Nuclio in production, apply both controls below.

### Authenticate the Dashboard API

By default, `dashboard.authConfig.kind` is unset, which selects the "NOP" authenticator — every request is accepted without credentials. The built-in authenticator kinds are listed in [`pkg/auth/types.go`](https://github.com/nuclio/nuclio/blob/development/pkg/auth/types.go); the non-NOP options are tied to specific platforms. For deployments that do not run on one of those platforms, terminate authentication at the network layer instead — for example with an authenticating ingress controller, an OAuth2 / OIDC proxy, or a service-mesh authorization policy — and ensure the Dashboard service only accepts traffic from that proxy.

Without an authenticator in front of the Dashboard, any client that can reach the Dashboard service (port 8070 by default) can read every function definition in the namespace — including the trigger credentials stored on those functions.

### Mask sensitive fields at rest

The Nuclio Kubernetes deployer can move sensitive trigger fields out of the `NuclioFunction` CRD and into Kubernetes Secrets, replacing the value in the CRD with a reference token. This feature is controlled by `platformConfig.sensitiveFields.maskSensitiveFields` and is **off by default** in the official Helm chart.

When the feature is off, credentials supplied to triggers (Kafka `password` and `attributes.sasl.password`, RabbitMQ `password`, `v3io-stream` `attributes.password`, Kinesis `attributes.secretAccessKey`, and so on) are stored verbatim in the `NuclioFunction` CRD and returned in plaintext by `GET /api/functions` and `GET /api/functions/{name}`.

Enable masking in your Helm values:

```yaml
# values.yaml
platformConfig:
  sensitiveFields:
    maskSensitiveFields: true
```

See [Sensitive fields](../../tasks/configuring-a-platform.md#sensitive-fields) in the platform configuration guide for the default match list and how to extend it with `customSensitiveFields`.

> **Note:** Masking applies on function deploy. Functions that already exist when you enable the feature need to be re-deployed for their credentials to migrate from the CRD into a Secret.

### Restrict the Dashboard's outbound requests

When a function is created with `spec.build.codeEntryType` set to `archive` (or with a bare URL supplied as `spec.build.path`), the Dashboard fetches that user-supplied URL **server-side** — the request originates from inside the cluster, from the Dashboard pod. Any client permitted to create a function can therefore cause the Dashboard to issue an HTTP `GET` to an arbitrary address, including cluster-internal services (the Kubernetes API server, other pods and `ClusterIP` services), the node loopback interface, and the cloud metadata endpoint (`169.254.169.254`). Differences in the returned error let the caller infer which internal ports are open. User-supplied `spec.build.codeEntryAttributes.headers` are forwarded on that request.

Nuclio treats function creation as a **privileged, trusted control-plane operation** — a function author already controls the function's image, environment, volumes, and mounts. The controls below keep that trust boundary intact in production:

1. **Authenticate the API and limit who can create functions** — see [Authenticate the Dashboard API](#securing-the-dashboard) above. This removes the unauthenticated path entirely.
2. **Restrict Dashboard ingress.** Allow traffic to the Dashboard service only from trusted sources:

    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: nuclio-dashboard-restrict-ingress
      namespace: nuclio
    spec:
      podSelector:
        matchLabels:
          nuclio.io/app: dashboard
      policyTypes: [Ingress]
      ingress:
        - from:
            - namespaceSelector:
                matchLabels:
                  kubernetes.io/metadata.name: nuclio
    ```

3. **Restrict Dashboard egress** so build downloads cannot reach internal ranges. Allow DNS and the registries or hosts you actually pull function code from, and deny the private RFC 1918 ranges, the link-local range (`169.254.0.0/16`, which covers the cloud metadata service), and the cluster API service IP. Egress `NetworkPolicy` enforcement requires a CNI that supports it, such as Calico or Cilium.
4. **On cloud nodes, enforce IMDSv2 with a hop limit of 1** so pods cannot reach the instance metadata service:

    ```bash
    aws ec2 modify-instance-metadata-options \
      --instance-id <id> --http-tokens required --http-put-response-hop-limit 1
    ```

> **Note:** These are deployment controls. If you delegate function creation to semi-trusted or multi-tenant users, the egress `NetworkPolicy` (control 3) is what prevents a permitted function author from reaching internal or metadata endpoints — authentication alone does not.

## Freezing a qualified version

When working in production, you need reproducibility and consistency.
It's therefore recommended that you don't use the latest stable version, but rather qualify a specific Nuclio version and "freeze" it in your configuration.
Stick with this version until you qualify a newer version for your system.
Because Nuclio adheres to backwards-compatibility standards between patch versions, and even minor version updates don't typically break major functionality, the process of qualifying a newer Nuclio version should generally be short and easy.

To use Helm to freeze a specific Nuclio version, set all of the `*.image.repository` and `*.image.tag` [Helm values](https://github.com/nuclio/nuclio/tree/development/hack/k8s/helm/nuclio/values.yaml) to the names and tags that represent the images for your chosen version.
Note the configured images must be accessible to your Kubernetes deployment (which is especially relevant for [air-gapped deployments](#air-gapped-deployment)).

## Air-gapped deployment

Nuclio is fully compatible with execution in air-gapped environments ("dark sites"), and supports the appropriate configuration to avoid any outside access.
The following guidelines refer to more advanced use cases and are based on the assumption that you can handle the related DevOps tasks.
Note that such implementations can get a bit tricky; to access a fully-managed, air-gap friendly, "batteries-included", Nuclio deployment, which also offers plenty of other tools and features, check out the enterprise-grade [Iguazio Data Science Platform](https://www.iguazio.com/platform/).
If you select to handle the implementation yourself, follow these guidelines; the referenced configurations are all [Helm values](https://github.com/nuclio/nuclio/tree/development/hack/k8s/helm/nuclio/values.yaml):

- Set `*.image.repository` and `*.image.tag` to [freeze a qualified version](#freezing-a-qualified-version), and ensure that the configured images are accessible to the Kubernetes deployment.
- Set `*.image.pullPolicy` to `Never` or to `IfNotPresent` to ensure that Kubernetes doesn't try to fetch the images from the web.
- Set `offline` to `true` to put Nuclio in "offline" mode.
- Set `dashboard.baseImagePullPolicy` to `Never`.
- Set `registry.pushPullUrl` to a registry URL that's reachable from your system.
- Ensure that base, "onbuild", and processor images are accessible to the dashboard in your environment, as they're required for the build process (either by `docker build` or [Kaniko](#using-kaniko-as-an-image-builder)).
  You can achieve this using either of the following methods:

  - Make the images available on the host Docker daemon (local cache).
  - Preload the images to a registry that's accessible to your system, to allow pulling the images from the registry.
    When using this method, set `registy.dependantImageRegistryURL` to the URL of an accessible local registry that contains the preloaded images (thus overriding the default location of `quay.io/nuclio`, which isn't accessible in air-gapped environments).
    <br/><br/>
    > **Note:** To save yourself some work, you can use the [pre-baked Nuclio registry](https://github.com/nuclio/prebaked-registry), either as-is or as a reference for creating your own local registry with preloaded images.

- To use the Nuclio templates library (optional), package the templates into an archive; serve the templates archive via a local server whose address is accessible to your system; and set `dashboard.templatesArchiveAddress` to the address of this local server.

<a id="using-kaniko-as-an-image-builder"></a>
## Using Kaniko as an image builder

When dealing with production deployments, you should avoid bind-mounting the Docker socket to the service pod of the Nuclio dashboard; doing so would allow the dashboard access to the host machine's Docker daemon, which is akin to giving it root access to your machine.
This is understandably a concern for real production use cases.
Ideally, no pod should access the Docker daemon directly, but because Nuclio is a container-based serverless framework, it needs the ability to build [OCI images](https://github.com/opencontainers/image-spec) at run time.
While there are several alternatives to bind-mounting the Docker socket, the selected Nuclio solution, starting with Nuclio version 1.3.15, is to integrate [Kaniko](https://github.com/GoogleContainerTools/kaniko) as a production-ready method of building OCI images in a secured way.
Kaniko is well maintained, stable, easy to use, and provides an extensive set of features.
Nuclio currently supports Kaniko only on Kubernetes.

To deploy Nuclio and direct it to use the Kaniko engine to build images, use the following [Helm values](https://github.com/nuclio/nuclio/tree/development/hack/k8s/helm/nuclio/values.yaml) parameters; replace the `<...>` placeholders with your specific values:

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
  Therefore, you must set `registry.pushPullUrl` to the URL of the registry to which Kaniko should push the resulting images, and in air-gapped environments, you must also set `registry.defaultBaseRegistryURL` and `registry.defaultOnbuildRegistryURL` to the URL of an accessible local registry that contains the preloaded base, "onbuild", and processor images (see [Air-gapped deployment](#air-gapped-deployment)).
- `quay.io` doesn't support nested repositories.
  If you're using Kaniko as a container builder and `quay.io` as a registry (`--set registry.pushPullUrl=quay.io/<repo name>`), add the following to your configuration to allow Kaniko caching to push successfully; (replace the `<repo name>` placeholder with the name of your repository):
    ```sh
    --set dashboard.kaniko.cacheRepo=quay.io/<repo name>/cache
    ```

### Using Kaniko with amazon elastic container registry (ECR):

To work with ECR, you must create a secret with your AWS credentials, and a secret with ECR Token while providing both secret names to the helm install command. 
This is relevant for instances running without attached IAM roles. 
To work with instances running with attached IAM roles, you can skip the AWS credentials and ECR Token secrets creation. 

Before you begin, make sure you have the following IAM roles attached to your user:

```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ecr:CreateRepository",
                "ecr:GetAuthorizationToken",
                "ecr:BatchCheckLayerAvailability",
                "ecr:BatchGetImage",
                "ecr:CompleteLayerUpload",
                "ecr:GetDownloadUrlForLayer",
                "ecr:InitiateLayerUpload",
                "ecr:PutImage",
                "ecr:UploadLayerPart"
            ],
            "Resource": "*"
        }
    ]
}
```
Common environment variables:
```shell
export AWS_REGION=<Your AWS region>
export AWS_ACCOUNT=<Your AWS account ID>
export ECR_PASSWORD=$(aws ecr get-login-password --region ${AWS_REGION})
```
Create the AWS credentials secret generated from `.aws/credentials` file configured with access key id and secret access key using the following command:
```shell
cat << EOF | kubectl --namespace nuclio create secret generic aws-credentials --save-config \
--dry-run=client --from-file=credentials=/dev/stdin -o yaml | kubectl apply -f -
[default]
aws_access_key_id = ${AWS_ACCESS_KEY_ID}
aws_secret_access_key = ${AWS_SECRET_ACCESS_KEY}
EOF
```
> **Note:** This is needed to allow Kaniko creating the image repository prior to pushing the function image. 
> Otherwise, Kaniko will fail to push the image to ECR because the image name is being determined during the build process.

Create the ECR token secret to be used as `imagePullSecret` of function pods.
Since ECR tokens go stale after 12 hours, the secret must be refreshed periodically (can be done with a cron job as described in [this blog post](https://skryvets.com/blog/2021/03/15/kubernetes-pull-image-from-private-ecr-registry/#update---aws-ecr-token-refresh))
```shell
kubectl -n nuclio create secret docker-registry ecr-registry-credentials \
  --docker-server=${AWS_ACCOUNT}.dkr.ecr.${AWS_REGION}.amazonaws.com \
  --docker-username=AWS \
  --docker-password=${ECR_PASSWORD} 
```

Finally, install the chart with the following command:
```shell
helm repo add nuclio https://nuclio.github.io/nuclio/charts
helm install nuclio \
    --set dashboard.kaniko.registryProviderSecretName=<aws-secret-name> \
    --set registry.secretName=<ecr-secret-name>
    --set registry.pushPullUrl=<your registry URL> \
    nuclio/nuclio
```

### Using Kaniko with cloud workload identity (Azure WI, GKE WI, AWS IRSA)

On managed Kubernetes you can authenticate Kaniko to your container registry via a cloud workload identity bound to the build pod's ServiceAccount, instead of mounting a static docker-config secret.
This is the standard pattern on AKS with Azure Workload Identity for ACR, on GKE with Workload Identity for Artifact Registry / GCR, and on EKS with IRSA for ECR.

Because Kaniko jobs are created by the Nuclio dashboard at run time (rather than rendered by the chart), the chart exposes `dashboard.kaniko.podLabels`, a map of labels that the dashboard applies to every build pod template it creates.
On clusters where workload identity is opt-in via a pod label (e.g. AKS requires `azure.workload.identity/use: "true"`), set those labels here.
You also need to set `dashboard.kaniko.defaultServiceAccount` (or the per-function `BuilderServiceAccount`) to a ServiceAccount that is bound to the cloud identity with push permissions on the target registry.

Example for AKS with Azure Workload Identity:

```sh
helm upgrade --install --reuse-values nuclio \
    --set registry.pushPullUrl=<your-acr>.azurecr.io \
    --set dashboard.containerBuilderKind=kaniko \
    --set dashboard.kaniko.defaultServiceAccount=<sa-bound-to-acrpush-identity> \
    --set-json 'dashboard.kaniko.podLabels={"azure.workload.identity/use":"true"}' \
    nuclio/nuclio
```

#### Precedence when both are configured

It's a valid configuration to set **both** `registry.secretName` (or `registry.credentials`) **and** `dashboard.kaniko.podLabels` + a workload-identity-bound `defaultServiceAccount`.
Kaniko's auth resolution is the standard go-containerregistry chain, so when a docker-config secret is mounted at `/kaniko/.docker/config.json`, **those static credentials take precedence** over the federated token that workload identity would otherwise provide.
The federated token is only consulted if the mounted config has no matching entry for the target registry.

This means the workload-identity setup is effectively shadowed when `registry.secretName` / `registry.credentials` is also set.
If you intend to authenticate to the registry via workload identity, do **not** also set `registry.secretName` (or only set one whose docker config does not match the target registry).
It is the responsibility of whoever configures Nuclio to choose one auth path or the other; the chart does not enforce this.
