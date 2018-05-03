# Setting up nuclio with AKS, Application Insights, and Grafana

#### In This Document
- [Application Insights overview](#application-insights-overview)
- [Create a new Application Insights account and obtain the instrumentation key](#create-a-new-application-insights-account-and-obtain-the-instrumentation-key)
- [Set up nuclio on Microsoft's Azure Container Service (AKS)](#set-up-nuclio-on-microsofts-azure-container-service-aks)
- [Send metrics telemetry to Azure Application Insights](#send-metrics-telemetry-to-azure-application-insights)
- [Configure nuclio Logger to send logs to Application Insights](#configure-nuclio-logger-to-send-logs-to-application-insights)
- [Visualize your Application Insights using Grafana](#visualize-your-application-insights-using-grafana)

## Application Insights overview

[Microsoft Azure Application Insights](https://azure.microsoft.com/en-us/services/application-insights/) is an extensible Application Performance Management (APM) service for web developers on multiple platforms. Use it to monitor your live web application. It will automatically detect performance anomalies. It includes powerful analytics tools to help you diagnose issues and to understand what users actually do with your app. It's designed to help you continuously improve performance and usability. It works for apps on a wide variety of platforms including .NET, Node.js and J2EE, hosted on-premises or in the cloud. For more information about Application Insights, see Microsoft's [product overview](https://docs.microsoft.com/en-us/azure/application-insights/app-insights-overview).

## Create a new Application Insights account and obtain the instrumentation key

See the [Application Insights documentation](https://docs.microsoft.com/en-us/azure/application-insights/app-insights-create-new-resource) for information on how to set up a new Application Insights account, and obtain your instrumentation key, as you'll use it later in the guide.

## Set up nuclio on Microsoft's Azure Container Service (AKS)

For detailed information on setting up in nuclio with Microsoft's [Azure Container Service (AKS)](https://azure.microsoft.com/services/container-service/), see  [Getting Started with nuclio on Azure Container Service (AKS)](getting-started-aks.md).

## Send metrics telemetry to Azure Application Insights

Nuclio abstracts the metrics sink. You inject into the `platform config` which metric implementation to use, and the nuclio internal communicates with the abstract layer of the metrics sink, agnostic to the implementation.

### Configuring the platform

In Kubernetes, a platform configuration is stored as a ConfigMap named `platform-config` in the namespace of the function. 

You'll create a ConfigMap in the nuclio namespace from a local file called `platform.yaml`.
Create a new file on your computer called `platform.yaml`. The system expects this specific name.

Place the following yaml code in this file:
```yaml
metrics:
  sinks:
    myAppInsights:
      kind: appinsights
      attributes:
        interval: 2s
        instrumentationKey: <YOUR-INSTUMENTATION-KEY-HERE>
        maxBatchSize: 1024
        maxBatchInterval: 5s
  system:
  - myAppInsights
  functions:
  - myAppInsights
```
The configuration makes use of the instrumentation key you obtained earlier in this guide.

Navigate to the location of your `platform.yaml` file and run the following `kubectl` command:
```sh
kubectl create configmap platform-config  --namespace nuclio --from-file platform.yaml
```

At this stage, all metrics will be sent to application insights custom metrics table.
To read more about platform configuration [click here](../../docs/tasks/configuring-a-platform.md)

## Configure nuclio Logger to send logs to Application Insights

The logger sink in configured in a similar way to the metrics sink.
Edit your `platform.yaml` file from the previous step,and append to it the following code:
```yaml
logger:
  sinks:
    stdout:
      kind: stdout
    myAppInsightsLogger:
      kind: appinsights
      attributes:
        instrumentationKey: <YOUR-INSTUMENTATION-KEY-HERE>
        maxBatchSize: 1024
        maxBatchInterval: 5s
  system:
  - level: debug
    sink: stdout
  - level: info
    sink: myAppInsightsLogger
  functions:
  - level: info
    sink: myAppInsightsLogger
```
The configuration makes use of the instrumentation key you obtained earlier in this guide.

Navigate to the location of your `platform.yaml` file and run the following `kubectl` command:
```sh
kubectl create configmap platform-config  --namespace nuclio --from-file platform.yaml
```

At this stage, all logs will be sent to application insights traces table.

For example, to use the logger in your function, you can simply add the following:
```go
context.Logger.InfoWith("Some message", "arg1", 1, "arg2", 2)
```

To read more about platform configuration [click here](../../docs/tasks/configuring-a-platform.md)

## Visualize your Application Insights using Grafana

[Grafana](https://grafana.com/) is the leading tool for querying and visualizing time series and metrics.

To use Grafana, you first need to install it in your cluster. 
You'll do this by using `helm`, the package manager for Kubernetes, and the [Grafana chart](https://hub.kubeapps.com/charts/stable/grafana).
If you are unfamiliar with `helm`, read more about it [here](https://docs.helm.sh/).

To allow Grafana to display data from Application Insights, The [Azure Monitor plugin](https://grafana.com/plugins/grafana-azure-monitor-datasource) developed by Grafana is required. 

To add the plugin, create a new file called `values.yaml`. Copy the following values to it, and edit the values such as `persistence`, `adminUser`, `adminPassword` and `plugins`.
```yaml
replicas: 1

image:
  repository: grafana/grafana
  tag: 5.0.4
  pullPolicy: IfNotPresent

downloadDashboardsImage:
  repository: appropriate/curl
  tag: latest
  pullPolicy: IfNotPresent
  
service:
  type: ClusterIP
  port: 80
  annotations: {}

ingress:
  enabled: false
  annotations: {}
    # kubernetes.io/ingress.class: nginx
    # kubernetes.io/tls-acme: "true"
  path: /
  hosts:
    - chart-example.local
  tls: []

resources: {}

nodeSelector: {}

tolerations: []

affinity: {}

persistence:
  enabled: true
  storageClassName: default
  accessModes:
    - ReadWriteOnce
  size: 10Gi

adminUser: admin
adminPassword: strongpassword

env: {}

plugins: "grafana-azure-monitor-datasource"

dashboardProviders: {}

dashboards: {}

grafana.ini:
  paths:
    data: /var/lib/grafana/data
    logs: /var/log/grafana
    plugins: /var/lib/grafana/plugins
  analytics:
    check_for_updates: true
  log:
    mode: console
  grafana_net:
    url: https://grafana.net

```

To install Grafana, run:
```
helm install stable/grafana --values values.yaml
```

Once the pod is up and running, you can access the web console. To do that, first find the pod name of Grafana:
```
kubectl get pods
```
Then run the following port-forward command to browse the web console:
```
kubectl --namespace default port-forward <REPLACE-WITH-GRAFANA-POD-NAME> 3000
```
Now, browse to http://127.0.0.1:3000/ and log in using the admin username and password you provided in the `values.yaml` file.

Verify that `Azure Monitor` exists in the plugins page.

Configure a data source using the [plugin support page](https://github.com/grafana/azure-monitor-datasource#configure-the-data-source).

Finally, see the provided sample Grafana JSON file (**[grafana-sample-dashboard.json](/docs/assets/grafana-sample-dashboard.json)**), which you can import from the Grafana dashboard: from the menu (plus icon - `+`) select **Create > Import** and upload the sample JSON file.

![Grafana Dashboard](/docs/assets/images/grafana-dashboard.jpg?raw=true "Grafana Dashboard")

### Further metric analysis using Application Insights

Go to your Application Insights account. You'll be able to query your tables for information. The query language is using Kusto.

Following are a few samples to quickly start querying:

```sql
customMetrics
| where name == "EventsHandleSuccessTotal" and timestamp > now(-1d) 
| extend trigger = tostring(customDimensions.TriggerID)
| summarize sum(value) by trigger
| render piechart 
```

```sql
customMetrics
| where name == "FunctionDuration" and timestamp > now(-1d) 
| extend workerIndex = tostring(customDimensions.WorkerIndex)
| project timestamp, value ,valueCount, workerIndex 
```

