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

Nuclio abstracts the metrics sync. You inject into the `platform config` which metric implementation to use, and the nuclio internal communicates with the abstract layer of the metrics sync, agnostic to the implementation.

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

Logger sync in configured in a similar way to the metrics sync.
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

To use Grafana, you first need to deploy it in your cluster. 
You'll do this by using `helm`, the package manager for Kubernetes, and the [Grafana chart](https://hub.kubeapps.com/charts/stable/grafana).
If you are unfamiliar with `helm`, read more about it [here](https://docs.helm.sh/).

To allow Grafana to pull data from Application Insights, you need to use a [special plugin](https://grafana.com/plugins/grafana-azure-monitor-datasource) developed by Grafana. 

In this tutorial you'll change the `values.yaml` file, to include the Application Insights plugin during installation. 

Create a new file called `values.yaml`. Copy the following values to it, and edit the values of your choice, such as `repository`, `tag`, `persistence`, `adminUser`, `adminPassword`.
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
## Expose the Grafana service to be accessed from outside the cluster (LoadBalancer service).
## or access it from within the cluster (ClusterIP service). Set the service type and the port to serve it.
## ref: http://kubernetes.io/docs/user-guide/services/
##
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
  #  - secretName: chart-example-tls
  #    hosts:
  #      - chart-example.local

resources: {}
#  limits:
#    cpu: 100m
#    memory: 128Mi
#  requests:
#    cpu: 100m
#    memory: 128Mi

## Node labels for pod assignment
## ref: https://kubernetes.io/docs/user-guide/node-selection/
#
nodeSelector: {}

## Tolerations for pod assignment
## ref: https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/
##
tolerations: []

## Affinity for pod assignment
## ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity
##
affinity: {}

## Enable persistence using Persistent Volume Claims
## ref: http://kubernetes.io/docs/user-guide/persistent-volumes/
##
persistence:
  enabled: true
  storageClassName: default
  accessModes:
    - ReadWriteOnce
  size: 10Gi
  # annotations: {}
  # subPath: ""
  # existingClaim:

adminUser: admin
adminPassword: strongpassword

## Extra environment variables that will be passed to deployment pods
env: {}

# Pass the plugins you want installed as a comma separated list.
# plugins: "digrich-bubblechart-panel,grafana-clock-panel"
plugins: "grafana-azure-monitor-datasource"

## Configure Grafana data sources
## ref: http://docs.grafana.org/administration/provisioning/#datasources
##
## Configure Grafana dashboard providers
## ref: http://docs.grafana.org/administration/provisioning/#dashboards
##
dashboardProviders: {}
#  dashboardproviders.yaml:
#    apiVersion: 1
#    providers:
#    - name: 'default'
#      orgId: 1
#      folder: ''
#      type: file
#      disableDeletion: false
#      editable: true
#      options:
#        path: /var/lib/grafana/dashboards

## Configure Grafana dashboard to import
## NOTE: To use dashboards you must also enable/configure dashboardProviders
## ref: https://grafana.com/dashboards
##
dashboards: {}
#  some-dashboard:
#    json: |
#      $RAW_JSON
#  prometheus-stats:
#    gnetId: 2
#    revision: 2
#    datasource: Prometheus

## Grafana's primary configuration
## NOTE: values in map will be converted to ini format
## ref: http://docs.grafana.org/installation/configuration/
##
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

Go to the location of the `values.yaml` file and run:
```
helm install stable/grafana --version 1.0.2 --values values.yaml
```

This will deploy Grafana in your cluster.

Once all the pods are up and running, you can access the web console. To do that, first find the pod name of Grafana:
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

