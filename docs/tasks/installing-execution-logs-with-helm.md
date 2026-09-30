# Installing Nuclio with Helm for Execution Logs

This guide covers what you need to do, specifically for the **Execution log** tab feature
(see [Viewing Function Execution Logs](viewing-execution-logs.md)), when installing or upgrading
Nuclio with Helm on Kubernetes.

## In This Document
- [Overview](#overview)
- [Step 1: Build and push custom images (until this ships in a release)](#step-1-build-and-push-custom-images)
- [Step 2: Create the required secrets](#step-2-create-the-required-secrets)
- [Step 3: Write a values file](#step-3-write-a-values-file)
- [Step 4: Install or upgrade the chart](#step-4-install-or-upgrade-the-chart)
- [Step 5: Verify](#step-5-verify)

<a id="overview"></a>
## Overview

The Execution log tab needs two things to work, neither of which is on by default:

1. **Nuclio images that include this feature.** This feature has not shipped in a tagged Nuclio
   release yet — as of writing, it only exists on the source branch it was developed on. Until it
   is merged and released, you'll need to build and push your own `dashboard` (and, to keep
   versions in lockstep, `controller`) images from that source, and point the Helm chart at them.
   Once it ships in a release, you can skip [Step 1](#step-1-build-and-push-custom-images)
   entirely and just use the stock image tags.
2. **An OpenSearch/Elasticsearch log source configured on the platform.** This is the same
   `kube.elasticSearchConfig` platform setting described in
   [Configuring a Platform](configuring-a-platform.md#elastic-search) — the Helm chart doesn't
   turn this on by itself, you set it via chart values as shown below.

<a id="step-1-build-and-push-custom-images"></a>
## Step 1: Build and push custom images (until this ships in a release)

> Skip this step once this feature is available in a tagged Nuclio release — just use the
> corresponding image tag instead.

From the root of your `nuclio/nuclio` checkout, on the branch/commit that has this feature:

```sh
make NUCLIO_ARCH=amd64 NUCLIO_OS=linux NUCLIO_LABEL=<your-nuclio-version> dashboard
```

This builds `$NUCLIO_DOCKER_REPO/dashboard:$NUCLIO_LABEL-<arch>`, Push the image to your registry

```sh
docker push $NUCLIO_DOCKER_REPO/dashboard:$NUCLIO_LABEL-amd64
```

<a id="step-2-create-the-required-secrets"></a>
## Step 2: Create the required secrets


**Elasticsearch/OpenSearch credentials**, so the dashboard can authenticate against your log
backend (use either a password or an API key, not both):

```sh
kubectl create secret generic nuclio-es-secret \
    --namespace nuclio \
    --from-literal=password='<your-elasticsearch-or-opensearch-password>'
```

<a id="step-3-write-a-values-file"></a>
## Step 3: Write a values file

Create a `values.yaml` for the install, filling in your registry and log-backend details:

```yaml
dashboard:
  image:
    repository: <your-registry>/nuclio/dashboard
    tag: execution-logs-amd64

  # points the dashboard at the Elasticsearch/OpenSearch password secret from Step 2
  elasticSearchPassword:
    secretName: nuclio-es-secret
    secretKey: password

platform:
  kube:
    elasticSearchConfig:
      url: <your-elasticsearch-or-opensearch-url>
      sslVerificationMode: none # or "full", if using a trusted certificate
      username: <your-username>
      index: <your-log-index-pattern> # e.g. "filebeat-*"
      customQueryParameter: "kubernetes.namespace.name:mlrun"
      # Optional safety net: prevents logs from leaking across projects if two functions in
      # different projects ever share a name. See "Viewing Function Execution Logs" for details.
      # projectNameField: kubernetes.labels.nuclio_io/project-name
```

<a id="step-4-install-or-upgrade-the-chart"></a>
## Step 4: Install or upgrade the chart

If this is a fresh install:

```sh
helm repo add nuclio https://nuclio.github.io/nuclio/charts
helm install nuclio nuclio/nuclio \
    --namespace nuclio \
    -f log_values.yaml
```

If you're enabling this on an existing Nuclio install, use `upgrade` instead:

```sh
helm upgrade nuclio nuclio/nuclio \
    --namespace nuclio \
    -f values.yaml
```

> While using custom images from [Step 1](#step-1-build-and-push-custom-images), install from your
> local chart checkout instead of the published `nuclio/nuclio` chart, by replacing `nuclio/nuclio`
> above with the path to `hack/k8s/helm/nuclio` in your `nuclio/nuclio` source tree.

<a id="step-5-verify"></a>
## Step 5: Verify

Port-forward the dashboard:

```sh
kubectl port-forward -n nuclio \
    $(kubectl get pods -n nuclio -l nuclio.io/app=dashboard -o jsonpath='{.items[0].metadata.name}') \
    8070:8070
```

Open `http://localhost:8070`, navigate to any function, and confirm the **Execution log** tab
appears immediately to the left of **Status**. If it doesn't, see
[Troubleshooting](viewing-execution-logs.md#troubleshooting) in the Execution logs guide.
