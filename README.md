[![Build Status](https://travis-ci.org/nuclio/nuclio.svg?branch=master)](https://travis-ci.org/nuclio/nuclio)
[![Go Report Card](https://goreportcard.com/badge/github.com/nuclio/nuclio)](https://goreportcard.com/report/github.com/nuclio/nuclio)
[![Slack](https://img.shields.io/badge/slack-join%20chat%20%E2%86%92-e01563.svg)](https://lit-oasis-83353.herokuapp.com/)

<p align="center"><img src="docs/images/logo.png" width="180"/></p>

# nuclio - "Serverless" for Real-Time Events and Data Processing

nuclio is a new serverless project, derived from iguazio's elastic data life-cycle management service for high-performance events and data processing. You can use nuclio as a standalone binary (for example, for IoT devices), package it within a Docker container or integrate it with a container orchestrator like Kubernetes.

nuclio is extremely fast. A single function instance can process hundreds of thousands of HTTP requests or data records per second. This is 10 - 100 times faster than some other frameworks. See [nuclio Architecture](docs/architecture.md) to learn how it works and watch a [technical presentation to the CNCF with demo](https://www.youtube.com/watch?v=xlOp9BR5xcs) (slides can be found [here](https://www.slideshare.net/iguazio/nuclio-overview-october-2017-80356865)).

**Note:** nuclio is still under active development and is not recommended for production use.

#### In This Document
* [Why Another "Serverless" Project?](#why-another-serverless-project)
* [Quick Start](#quick-start)
* [High-Level Architecture](#high-level-architecture)
* [Function Examples](#function-examples)
* [Further Reading](#further-reading)

## Why Another "Serverless" Project?

We considered existing cloud and open-source serverless solutions, but none addressed our needs:

* Real-time processing with minimal CPU and I/O overhead and maximum parallelism
* Native integration with a large variety of data sources, triggers, and processing models
* Abstraction of data resources from the function code to support code portability, simplicity and data-path acceleration
* Simple debugging, regression testing and multi-versioned CI/CD pipelines
* Portability across low-power devices, laptops, on-prem clusters and public clouds

We designed nuclio to be extendable, using a modular and layered approach - constantly adding triggers and data sources.  We hope many will join us in developing new modules, developer tools and platforms.

## Quick Start

The simplest way to explore nuclio is to run the nuclio playground (you only need docker):

```bash
docker run -p 8070:8070 -v /var/run/docker.sock:/var/run/docker.sock -v /tmp:/tmp nuclio/playground
```

![playground](docs/images/playground.png)

Browse to http://localhost:8070, deploy one of the example functions or write your own. The playground, when run outside of an orchestration platform (e.g. Kubernetes or Swarm), will simply deploy to the local docker daemon.

The [Getting Started With nuclio On Kubernetes](docs/k8s/getting-started.md) guide has a complete step-by-step guide to using nuclio over Kubernetes both with the playground and `nuctl` - nuclio's command line interface.

## High-Level Architecture

![architecture](docs/images/architecture.png)

### Services

#### Processor
A processor listens on one or more triggers (for example, HTTP, Message Queue, Stream), and executes user functions with one or more parallel workers.

The workers use language-specific runtimes to execute the function (via native calls, shared memory, or shell). Processors use abstract interfaces to integrate with platform facilities for logging, monitoring and configuration, allowing for greater portability and extensibility (such as logging to a screen, file, or log stream).

#### Controller
A controller accepts function and event-source specifications, invokes builders and processors through an orchestration platform (such as Kubernetes), and manages function elasticity, life cycle, and versions.

#### Playground
The playground is a standalone container micro-service accessed through HTTP, it presents a code editor UI for editing, deploying and testing functions. This is the most user-friendly way to work with nuclio. The playground container comes with a version of the builder inside.

#### Builder
A builder receives raw code and optional build instructions and dependencies, and generates the function artifact - a binary file or a Docker container image, which the builder can also push to a specified image repository. The builder can run in the context of the CLI or as a separate service for automated development pipelines.

#### Dealer
A dealer is used with streaming and batch jobs to distribute a set of tasks or data partitions/shards among the available function instances, and guarantee that all tasks are completed successfully. For example, if a function reads from a message stream with 20 partitions, the dealer will guarantee that the partitions are distributed evenly across workers, taking into account the number of function instances and failures.

### Function Concepts

#### Triggers
Functions can be invoked through a variety of event sources (such as HTTP, RabitMQ, Kafka, Kinesis, NATS, DynamoDB, iguazio v3io, or schedule), which are defined in the function specification. Event sources are divided into several event classes (req/rep, async, stream, pooling), which define the sources' behavior. Different event sources can plug seamlessly into the same function without sacrificing performance, allowing for portability, code reuse, and flexibility.

#### Data bindings
Data-binding rules allow users to specify persistent input/output data resources to be used by the function. (data connections are preserved between executions). Bound data can be in the form of files, objects, records, messages etc. The function specification may include an array of data-binding rules, each specifying the data resource and its credentials and usage parameters. Data-binding abstraction allows using the same function with different data sources of the same type, and enables function portability.

#### SDK
The nuclio SDK is used by function developers to write, test, and submit their code, without the need for the entire nuclio source tree.

For more information about the nuclio architecture, see [nuclio Architecture](docs/architecture.md).

## Function Examples

The function demonstrated below uses the `Event` and `Context` interfaces to handle inputs and logs, returining a structured HTTP response (can also use a simple string as returned value).

in Golang
```golang
package handler

import (
    "github.com/nuclio/nuclio-sdk"
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

in Python
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

More examples can be found [here](hack/examples/README.md).

## Further Reading

* Getting started:
    * [Getting Started With nuclio On Kubernetes](docs/k8s/getting-started.md)
    * Getting Started With nuclio On Raspberry Pi (coming soon)
* Guides and examples:
    * [Configuring a function](docs/configuring-a-function.md)
    * [Function Examples](hack/examples/README.md)
    * [nuctl Reference](docs/nuctl/nuctl.md)
    * Kubernetes
        * [Invoking functions by name with an ingress](docs/k8s/function-ingress.md)
* Under the hood:
    * [Architecture Details](docs/architecture.md)
    * Build Process (coming soon)
    * Deploy Process (coming soon)
* Media:
    * [nuclio and the Future of Serverless Computing](https://thenewstack.io/whats-next-serverless/)
    * [nuclio: The New Serverless Superhero](https://hackernoon.com/nuclio-the-new-serverless-superhero-3aefe1854e9a)

For more questions and help, feel free to join the friendly [nuclio slack channel](https://lit-oasis-83353.herokuapp.com).
