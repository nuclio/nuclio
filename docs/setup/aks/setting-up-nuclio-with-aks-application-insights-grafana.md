# Setting up nuclio with AKS, Application Insights and Grafana

## Abstract
In this guide we will show how to: 
1. Setup your nuclio platform on AKS, 
2. Configure it to send metrics telemetry to Azure Application Insights,
3. Configure nuclio Logger to send logs to Application Insights,
4. Display all the telemtry send to Application Insight in grafana.

## Application Insights
Application Insights is an extensible Application Performance Management (APM) service for web developers on multiple platforms. Use it to monitor your live web application. It will automatically detect performance anomalies. It includes powerful analytics tools to help you diagnose issues and to understand what users actually do with your app. It's designed to help you continuously improve performance and usability. It works for apps on a wide variety of platforms including .NET, Node.js and J2EE, hosted on-premises or in the cloud. 
[Click here](https://docs.microsoft.com/en-us/azure/application-insights/app-insights-overview) to read more about Application Insights.

## How to create a new Application Insight account, and obtain the `Instrumentation key`?
Please [click here](https://docs.microsoft.com/en-us/azure/application-insights/app-insights-create-new-resource) to read how to setup a new Application Insights account, and obtain your instrumentation key, as we will use it later in the guide.

## Setting up nuclio on Microsoft's [Azure Container Service (AKS)](https://azure.microsoft.com/services/container-service/)

We covered this section in details in our [Getting Started with nuclio on Azure Container Service (AKS)](getting-started-aks.md)

## Sending metrics telemetry to Azure Application Insights

Nuclio abstracts out the metrics sync. We inject the  `platform config` which metric implementation to use, and the nuclio internal communicates with the abstract layer of the metrics sync, agnostic to the implementation.

### Platform config
In Kubernetes, a platform configuration is stored as a configmap named `platform-config` in the namespace of the function. 

We will create a configmap in the nuclio namespace from a local file called `platform.yaml`.
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
The configuration makes use of the instrumentation key you obstained eralier in this guide.

Navigate to the location of your `platform.yaml` file and use kubectl to run:
```
kubectl create configmap platform-config  --namespace nuclio --from-file platform.yaml
```
At this point, all metrics will be sent to application insights custom metrics table.
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
The configuration makes use of the instrumentation key you obstained eralier in this guide.

Navigate to the location of your `platform.yaml` file and use kubectl to run:
```
kubectl create configmap platform-config  --namespace nuclio --from-file platform.yaml
```
At this point, all logs will be sent to application insights traces table.

For example, to use the logger in your function, you can simply:
```go
context.Logger.InfoWith("Some message", "arg1", 1, "arg2", 2)
```

To read more about platform configuration [click here](../../docs/tasks/configuring-a-platform.md)

## Visualize your application insight using Grafana
[Grafana](https://grafana.com/) is the leading tool for querying and visualizing time series and metrics.

To use Grafana we first need to deploy it in your cluster. 
We will do that using `helm`, The package manager for Kubernetes, and the [Grafana chart](https://hub.kubeapps.com/charts/stable/grafana).
If you are unfamiliar with `helm`, please [click here](https://docs.helm.sh/) to read more about it.

In order for Grafana to pull data from Application Insights, we need to use a [special plugin](https://grafana.com/plugins/grafana-azure-monitor-datasource) developed by Grafana. 

In this tutorial we will change the `values.yaml` file, to include the Application Insights plugin during installation. 

Create a new file called `values.yaml`. Copy the following values to it, and edit the values of your choice, such as: repository, tag, persitance, adminUser, adminPassword.
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
## Expose the grafana service to be accessed from outside the cluster (LoadBalancer service).
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


## Extra environment variables that will be pass onto deployment pods
env: {}

# Pass the plugins you want installed as a comma separated list.
# plugins: "digrich-bubblechart-panel,grafana-clock-panel"
plugins: "grafana-azure-monitor-datasource"

## Configure grafana datasources
## ref: http://docs.grafana.org/administration/provisioning/#datasources
##
## Configure grafana dashboard providers
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

## Configure grafana dashboard to import
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

Once all the pods are up and running, we can access the web console. To do that, first find the pod name of Grafana:
```
kubectl get pods
```
Then run the following port-forward command to browse the web console:
```
kubectl --namespace default port-forward <REPLACE-WITH-GRAFANA-POD-NAME> 3000
```
Now browse to: [http://127.0.0.1:3000/](http://127.0.0.1:3000/) and login using the admin username and password you provided in the `values.yaml` file.

Verify that `Azure Monitor` exists in the plugins page.

Configure a datasource using the [plugin support page](https://github.com/grafana/azure-monitor-datasource#configure-the-data-source).

Finally, we have [included a sample dashboard which you can import](grafana-sample-dashboard.json). 
Go to Menu (plus icon)-> Create-> Import

Click the import .json file button to upload the included sample.

![Grafana Dashboard](grafana.jpg?raw=true "Grafana Dashboard")

### Further metric anlysis using Application Insights
Go to your Application Insights account.
You will be able to query your tables for information.
The query language is using Kusto.

We have provided few samples to quickly start query:

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



