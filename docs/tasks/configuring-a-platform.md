# Configuring a Platform

#### In This Document
- [Overview](#overview)
- [Creating a platform configuration in Kubernetes](#k8s-platform-config-create)
- [Configuration elements](#configuration-elements)

## Overview

Function configuration carries information about the specific function (how it's triggered, the runtime type, etc.) whereas a platform configuration carries information about the platform on which functions are run. For example, where should the function log to? What sort of metric mechanism is in place? Which port should the function listen on for health checks? 

While this could theoretically be passed in the function configuration, it would make configuration updates a complex task of regenerating the configuration for all provisioned functions. The platform configuration is therefore stored separately, shared amongst all functions that share a platform.

> **Note:** A "platform" could be a cluster or any sub resource of that cluster like a namespace. If, for example, you have a namespace per tenant, you configure logging, metrics, etc. differently for each tenant

<a id="k8s-platform-config-create"></a>
## Creating a platform configuration in Kubernetes

In Kubernetes, a platform configuration is stored as a ConfigMap named `platform-config` in the namespace of the function. For example, to create a ConfigMap in the "nuclio" namespace from a local file called `platform.yaml`, run the following from a command line:
```sh
kubectl create configmap platform-config  --namespace nuclio --from-file platform.yaml
```

<a id="config-elements"></a>
## Configuration elements

<a id="logger-supported-log-sinks"></a>
### Log sinks (`logger`)

Configuring where a function logs to is a two step process. First, you create a named logger sink and provide it with configuration. Then, you reference this logger sink at the desired scope with a given log level. Scopes include the following:

- **System logging** - This is where logs from services like the controller, the dashboard, etc. are shipped to
- **Function logging** - Unless overridden per function, this is where the function logs are shipped to
- **A specific function** - An optional override per function, allowing specific functions to ship elsewhere than the platform function logger

Let's say you want to ship all function logs and only warning/error logs from the system to Azure App Insights. However, you want all system logs to also go to `stdout`. Your `logger` section in the `platform.yaml` would look like this:

```yaml
logger:
  sinks:
    myStdoutLogger:
      kind: stdout
    myAppInsightsLogger:
      kind: appinsights
      attributes:
        instrumentationKey: something
        maxBatchSize: 512
        maxBatchInterval: 10s
  system:
  - level: debug
    sink: myStdoutLogger
  - level: warning
    sink: myAppInsightsLogger
  functions:
  - level: debug
    sink: myAppInsightsLogger
```

First, you declared the two sinks: `myStdoutLogger` and `myAppInsightsLogger`. Then, you bound `system:debug` (which catches all logs at the severity level and higher) to `myStdoutLogger`, and `system:warning`, `functions:debug` to `myAppInsightsLogger`.

<a id="supported-log-sinks"></a>
#### Supported log sinks

All log sinks support the following fields:

- `kind` - The kind of output
- `url` - The URL at which the sink resides
- `attributes` - Kind specific attributes

<a id="log-sink-stdout"></a>
##### Standard output (`stdout`)

The standard output sink currently does not support any specific attributes.

<a id="log-sink-appinsights"></a>
##### Azure Application Insights (`appinsights`)

- `attributes.instrumentationKey` - The instrumentation key from Azure
- `attributes.maxBatchSize` - Max number of records to batch together before sending to Azure (defaults to 1024)
- `attributes.maxBatchInterval` - Time to wait for maxBatchSize records (valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`), after which whatever is gathered will be sent towards Azure (defaults to `3s`)

<a id="metrics"></a>
### Metric sinks (`metrics`)

Metric sinks behave similarly to logger sinks in that first you declare a sink and then bind a scope to it. To illustrate with an example, if you would (for some reason) want all of your system metrics to be pulled by Prometheus whereas all function metrics pushed to a Prometheus push proxy, your `metrics` section in the `platform.yaml` would look like this:

```yaml
metrics:
  sinks:
    myPromPush:
      kind: prometheusPush
      url: http://prometheus-prometheus-pushgateway:9091
      attributes:
        jobName: myPushJob
        instanceName: myPushInstance
        interval: 10s
    myPromPull:
      kind: prometheusPull
      url: :8090
      attributes:
        jobName: myPullJob
        instanceName: myPullInstance
    myAppInsights:
      kind: appisights
      attributes:
        interval: 10s
        instrumentationKey: something
        maxBatchSize: 2048
        maxBatchInterval: 60s
  system:
  - myPromPull
  - myAppInsights
  functions:
  - myPromPush
```

<a id="supported-metric-sinks"></a>
#### Supported metric sinks

All metric sinks support the following fields:

- `kind` - The kind of output
- `url` - The URL at which the sink resides
- `attributes` - Kind specific attributes

<a id="metric-sink-prometheusPush"></a>
##### Prometheus push (`prometheusPush`)

- `url` - The URL at which the push proxy resides
- `attributes.jobName` - The Prometheus job name
- `attributes.instanceName` - The Prometheus instance name
- `attributes.interval` - A string holding the interval to which the push occurs such as `10s`, `1h` or `2h45m`.
    Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`

<a id="metric-sink-prometheusPull"></a>
##### Prometheus pull (`prometheusPull`)

- `url` - The URL at which the HTTP listener serves pull requests
- `attributes.jobName` - The Prometheus job name
- `attributes.instanceName` - The Prometheus instance name

<a id="metric-sink-appinsights"></a>
##### Azure Application Insights (`appinsights`)

- `attributes.interval` - A string holding the interval to which the push occurs such as `10s`, `1h` or `2h45m`. Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`
- `attributes.instrumentationKey` - The instrumentation key from Azure
- `attributes.maxBatchSize` - Max number of records to batch together before sending to Azure (defaults to 1024)
- `attributes.maxBatchInterval` - Time to wait for maxBatchSize records (valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`), after which whatever is gathered will be sent towards Azure (defaults to `3s`)

<a id="webAdmin"></a>
### Webadmin (`webAdmin`)

Functions can optionally serve requests to get and update their configuration via HTTP. By default this is enabled at address `:8081` but can be overridden by the configuration:

- `enabled` - Whether or not to listen to requests. `true`, by default
- `listenAddress` - The address to listen on. `:8081`, by default

For example, the following configuration can be used listen on port `:10000`:

```yaml
webAdmin:
  listenAddress: :10000
```

<a id="healthCheck"></a>
### Health check (`healthCheck`)

An important part of the function life cycle is to verify its health via HTTP. By default this is enabled at address `:8082` but can be overridden by the configuration:

- `enabled` - Whether or not to listen to requests. `true`, by default
- `listenAddress` - The address to listen on. `:8082`, by default

For example, the following configuration disables responses to health checks:

```yaml
healthCheck:
  enabled: false
```

<a id="cronTriggerCreationMode"></a>
### Cron-trigger creation mode (`cronTriggerCreationMode`)

The `cronTriggerCreationMode` configuration field determines how to run Cron triggers:

- `"processor"` (default) - Run Cron triggers from the Nuclio processor.
- `"kube"` - **[Tech Preview]** Run Cron triggers as Kubernetes CronJobs; applicable only on Kubernetes platforms.

For example, the following configuration implements Cron triggers as Kubernetes CronJobs on a Kubernetes platform:
```yaml
cronTriggerCreationMode: "kube"
```

For more information, see the [Cron-trigger reference](../reference/triggers/cron.md).

<a id="runtime"></a>
### Runtime (`runtime`)

The `runtime` sections allows you to configure various runtime related parameters. 
For example to define custom PyPI repository, add the following section:
```yaml
  runtime:
    python:
      buildArgs:
        PIP_INDEX_URL: "https://test.pypi.org/simple"
```

#### envFrom (`runtime.common.envFrom`)

`envFrom` is a configuration of [`[]v1.envFromSource`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.23/#envfromsource-v1-core) type 
which enables the specification of secrets/configMaps for propagation to all functions. Platform values take lower precedence 
compared to function config values, which can be specified in the function config spec under `envFrom` ([function configuration](../reference/function-configuration) documentation).
```yaml
  envFrom:
    - secretRef:
        name: test-secret
```

<a id="sensitive-fields"></a>
### Sensitive fields

Nuclio resources (currently function configuration and api gateway configuration) may contain sensitive information such as passwords, tokens, etc.
When the 'masking sensitive fields' feature is enabled, these fields get obfuscated, and their raw values are stored separately (in a Kubernetes secret). They are then populated internally when needed, during function deployment or api gateway creation.

In api gateway config only `password` field is masked if 'masking sensitive fields' is enabled.
For function config there are some config fields that are [masked by default](https://github.com/nuclio/nuclio/blob/development/pkg/platformconfig/types.go#L303-L340). You can add custom sensitive fields to mask by specifying the regex to the path in the function configuration.
The masked fields are replaced with references (`$ref`) in the function configuration.
Example:
```yaml
  sensitiveFields:
    maskSensitiveFields: true
    customSensitiveFields:
    - "^/spec/triggers/.+/url$"
```
