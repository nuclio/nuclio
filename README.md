![Periodic](https://github.com/nuclio/nuclio/workflows/Periodic/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/nuclio/nuclio)](https://goreportcard.com/report/github.com/nuclio/nuclio)
[![Slack](https://img.shields.io/badge/slack-join%20chat%20%E2%86%92-e01563.svg)](https://lit-oasis-83353.herokuapp.com/)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/nuclio)](https://artifacthub.io/packages/search?repo=nuclio)
[![Iguazio Careers](https://img.shields.io/badge/careers-We're%20Hiring!-informational?style=square-flat-square)](https://www.iguazio.com/careers)

<p align="center"><img src="/docs/assets/images/logo.png" width="180" alt="nuclio"/></p>

# Nuclio - "Serverless" for Real-Time Events and Data Processing

<p align="center">
Visit <a href="https://nuclio.io">nuclio.io</a> for product information and news and a friendly web presentation of the Nuclio <a href="https://nuclio.io/docs/latest/">documentation</a>.
</p>

Translations: 

- [简体中文](./README_zh.md)

#### In this document

- [Overview](#overview)
- [Why another "serverless" project?](#why-another-serverless-project)
- [Quick-start steps](#quick-start-steps)
- [How it works](#how-it-works)
- [Function examples](#function-examples)
- [Further reading](#further-reading)

## Overview

Nuclio is a high-performance "serverless" framework focused on data, I/O, and compute intensive workloads. It is well integrated with popular data science tools, such as [Jupyter](https://jupyter.org/) and [Kubeflow](https://www.kubeflow.org/); supports a variety of data and streaming sources; and supports execution over CPUs and GPUs. The Nuclio project began in 2017 and is constantly and rapidly evolving; many start-ups and enterprises are now using Nuclio in production.

You can use Nuclio as a standalone Docker container or on top of an existing [Kubernetes](https://kubernetes.io) cluster; see the deployment instructions in the Nuclio documentation. You can also use Nuclio through a fully managed application service (in the cloud or on-prem) in the [Iguazio Data Science Platform](https://www.iguazio.com/), which you can [try for free](https://go.iguazio.com/start-your-free-trial).

If you wish to create and manage Nuclio functions through code - for example, from Jupyter Notebook - see the [Nuclio Jupyter project](https://github.com/nuclio/nuclio-jupyter), which features a Python package and SDK for creating and deploying Nuclio functions from Jupyter Notebook.
Nuclio is also an integral part of the new open-source [MLRun](https://github.com/mlrun/mlrun) library for data science automation and tracking and of the open-source [Kubeflow Pipelines](https://www.kubeflow.org/docs/components/pipelines/) framework for building and deploying portable, scalable ML workflows.

Nuclio is extremely fast: a single function instance can process hundreds of thousands of HTTP requests or data records per second.
This is 10-100 times faster than some other frameworks. To learn more about how Nuclio works, see the Nuclio [architecture](/docs/concepts/architecture.md) documentation, read this review of [Nuclio vs. AWS Lambda](https://theburningmonk.com/2019/04/comparing-nuclio-and-aws-lambda/), or watch the [Nuclio serverless and AI webinar](https://www.youtube.com/watch?v=pTCx569Kd4A).
You can find links to additional articles and tutorials on the [Nuclio web site](https://nuclio.io/).

Nuclio is secure: Nuclio is integrated with [Kaniko](https://github.com/GoogleContainerTools/kaniko) to allow a secure and production-ready way of building Docker images at run time.

For further questions and support, [click to join](https://lit-oasis-83353.herokuapp.com) the [Nuclio Slack](https://nuclio-io.slack.com) workspace.

## Why another "serverless" project?

None of the existing cloud and open-source serverless solutions addressed all the desired capabilities of a serverless framework:

- Real-time processing with minimal CPU/GPU and I/O overhead and maximum parallelism
- Native integration with a large variety of data sources, triggers, processing models, and ML frameworks
- Stateful functions with data-path acceleration
- Portability across low-power devices, laptops, edge and on-prem clusters, and public clouds
- Open-source but designed for the enterprise (including logging, monitoring, security, and usability)

Nuclio was created to fulfill these requirements.  It was intentionally designed as an extendable open-source framework, using a modular and layered approach that supports constant addition of triggers and runtimes, with the hope that many will join the effort of developing new modules, developer tools, and platforms for Nuclio.

## Quick-start steps

The simplest way to explore Nuclio is to run its graphical user interface (GUI) of the Nuclio dashboard. All you need to run the dashboard is Docker:

```sh
docker run -p 8070:8070 -v /var/run/docker.sock:/var/run/docker.sock -v /tmp:/tmp --name nuclio-dashboard quay.io/nuclio/dashboard:stable-amd64
```

![dashboard](/docs/assets/images/dashboard.png)

Browse to http://localhost:8070, create a project, and add a function. When run outside of an orchestration platform (for example, Kubernetes), the dashboard will simply deploy to the local Docker daemon.

Assuming you are running Nuclio with Docker, as an example, create a project and deploy the pre-existing template "dates (nodejs)".
With `docker ps`, you should see that the function was deployed in its own container.
You can then invoke your function with curl; (check that the port number is correct by using `docker ps` or the Nuclio dashboard):

```sh
curl -X POST -H "Content-Type: application/text" -d '{"value":2,"unit":"hours"}' http://localhost:37975
```

For a complete step-by-step guide to using Nuclio over Kubernetes, either with the dashboard UI or the Nuclio command-line interface (`nuctl`), explore these learning pathways:

- [Getting Started with Nuclio on Kubernetes](/docs/setup/k8s/getting-started-k8s.md)
- [Getting Started with Nuclio on Google Kubernetes Engine (GKE)](/docs/setup/gke/getting-started-gke.md)
- [Getting started with Nuclio on Azure Container Services (AKS)](/docs/setup/aks/getting-started-aks.md)
- [Hands-on live Kubernetes sandbox and guiding instructions for Nuclio, free on Katacoda](https://katacoda.com/javajon/courses/kubernetes-serverless/nuclio)

## How it works

"When this happens, do that". Nuclio tries to abstract away all the scaffolding around taking an event that occurred (e.g. a record was written into Kafka, an HTTP request was made, a timer expired) and passing this information to a piece of code for processing. To do this, Nuclio expects the users to provide (at the very least) information about what can trigger an event and the code to run when such an event happens. Users provide this information to Nuclio either via the command line utility (`nuctl`), a REST API or visually through a web application. 

![architecture](/docs/assets/images/architecture-3.png)

Nuclio takes this information (namely, the function `handler` and the function `configuration`) and sends it to a builder. This builder will craft the function's container image holding the user's handler and a piece of software that can execute this handler whenever events are received (more on that in a bit). The builder will then "publish" this container image by pushing it to a container registry.

Once published, the function container image can be deployed. The deployer will craft orchestrator specific configuration from the function's configuration. For example, if deploying to Kubernetes the deployer will take configuration parameters like number of replicas, auto scaling timing parameters, how many GPUs the function is requesting and convert this to Kubernetes resource configuration (i.e. Deployment, Service, Ingress, etc). 

> Note: The deployer does not create Kubernetes native resources directly, but rather creates a "NuclioFunction" custom resource (CRD). A Nuclio service called the "controller" listens to changes on the NuclioFunction CRD and creates/modifies/destroys the applicable Kubernetes native resources (Deployment, Service, etc). This follows the standard Kubernetes operator pattern

The orchestrator will then spin up containers from the published container images and execute them, providing them the function configuration. The entrypoint of these containers is the "processor", responsible for reading the configuration, listening to event triggers (e.g. connecting to Kafka, listening for HTTP), reading events when they happen and calling the user's handler. The processor is responsible for many, many other things including handling metrics, marshaling responses, gracefully handling crashes, etc. 

### Scaling to Zero

Once built and deployed to an orchestrator like Kubernetes, Nuclio functions (namely, processors) can process events, scale up and down based on performance metrics, ship logs and metrics - all without the help of any external entity. Once deployed, you can terminate the Nuclio Dashboard and Controller services and Nuclio functions will still run and scale perfectly.

However, scaling to zero is not something they can do on their own. Rather - once scaled to zero, a Nuclio function cannot scale itself up when a new event arrives. For this purpose, Nuclio has a "Scaler" service. This handles all matters of scaling to zero and, more importantly, from zero.  

## Function examples

The following sample function implementation uses the `Event` and `Context` interfaces to handle inputs and logs, returning a structured HTTP response; (it's also possible to use a simple string as the returned value).

In Go
```golang
package handler

import (
    "github.com/nuclio/nuclio-sdk-go"
)

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
    context.Logger.Info("Request received: %s", event.GetPath())

    return nuclio.Response{
        StatusCode:  200,
        ContentType: "application/text",
        Body: []byte("Response from handler"),
    }, nil
}
```

In Python
```python
def handler(context, event):
    response_body = f'Got {event.method} to {event.path} with "{event.body}"'

    # log with debug severity
    context.logger.debug('This is a debug level message')

    # just return a response instance
    return context.Response(body=response_body,
                            headers=None,
                            content_type='text/plain',
                            status_code=201)
```

More examples can be found in the **[hack/examples](hack/examples/README.md)** Nuclio GitHub directory.

## Further reading

- Setup
  - [Getting Started with Nuclio on Docker](/docs/setup/docker/getting-started-docker.md)
  - [Getting Started with Nuclio on Minikube](/docs/setup/minikube/getting-started-minikube.md)
  - [Getting Started with Nuclio on Kubernetes](/docs/setup/k8s/getting-started-k8s.md)
  - [Getting Started with Nuclio on Azure Kubernetes Service (AKS)](/docs/setup/aks/getting-started-aks.md)
  - [Getting Started with Nuclio on Google Kubernetes Engine (GKE)](/docs/setup/gke/getting-started-gke.md)
  - Getting Started with Nuclio on Raspberry Pi (coming soon)
- Tasks
  - [Deploying Functions](/docs/tasks/deploying-functions.md)
  - [Deploying Functions from Dockerfile](/docs/tasks/deploy-functions-from-dockerfile.md)
  - [Deploying Pre-Built Functions](/docs/tasks/deploying-pre-built-functions.md)
  - [Configuring a Platform](/docs/tasks/configuring-a-platform.md)
- Concepts
  - [Best Practices and Common Pitfalls](/docs/concepts/best-practices-and-common-pitfalls.md)
  - [Architecture](/docs/concepts/architecture.md)
  - Kubernetes
    - [Invoking Functions by Name with a Kubernetes Ingress](/docs/concepts/k8s/function-ingress.md)
- References
  - [nuctl](/docs/reference/nuctl/nuctl.md)
  - [Function-Configuration Reference](/docs/reference/function-configuration/function-configuration-reference.md)
  - [Triggers](/docs/reference/triggers)
  - [Runtime - .NET Core 3.1](/docs/reference/runtimes/dotnetcore/writing-a-dotnetcore-function.md)
  - [Runtime - Shell](/docs/reference/runtimes/shell/writing-a-shell-function.md)
- [Examples](hack/examples/README.md)
- Sandbox
  - [Install Nuclio and run functions. Explore and experiment on a free Kubernetes cluster.](https://katacoda.com/javajon/courses/kubernetes-serverless/nuclio)
- Contributing
  - [Code conventions](/docs/devel/coding-conventions.md)
  - [Contributing to Nuclio](/docs/devel/contributing.md)
- Media
  - [Running High-Speed Serverless with nuclio (slides)](https://www.slideshare.net/iguazio/running-highspeed-serverless-with-nuclio)
  - [CNCF Webinar – Serverless and AI (video)](https://www.youtube.com/watch?v=pTCx569Kd4A)
  - [Faster AI Development With Serverless (tutorial)](https://dzone.com/articles/tutorial-faster-ai-development-with-serverless)
  - [nuclio and the Future of Serverless Computing (blog)](https://thenewstack.io/whats-next-serverless/)
  - [nuclio: The New Serverless Superhero (blog)](https://hackernoon.com/nuclio-the-new-serverless-superhero-3aefe1854e9a)
  - [Serverless Framework for Real-Time Apps Emerges (Blog)](https://www.rtinsights.com/serverless-framework-for-real-time-apps-emerges/)

For support and additional product information, [join](https://lit-oasis-83353.herokuapp.com) the active [Nuclio Slack](https://nuclio-io.slack.com) workspace.
