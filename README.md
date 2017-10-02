[![Build Status](https://travis-ci.org/nuclio/nuclio.svg)](https://travis-ci.org/nuclio/nuclio)
[![Go Report Card](https://goreportcard.com/badge/github.com/nuclio/nuclio)](https://goreportcard.com/report/github.com/nuclio/nuclio)

<p align="center"><img src="docs/images/logo.png" width="180"/></p>

# nuclio &mdash; "Serverless" for Real-Time Events and Data Processing

nuclio is a new serverless project, derived from iguazio's elastic data life-cycle management service for high-performance events and data processing.
nuclio is being extended to support a large variety of event and data sources.
You can use nuclio as a standalone binary (for example, for IoT devices), package it within a Docker container, or integrate it with a container orchestrator like Kubernetes.

nuclio is extremely fast. A single function instance can process hundreds of thousands of HTTP requests or data records per second.
This is 10&ndash;100 times faster than some other frameworks. See [nuclio Architecture](docs/architecture.md) to learn how it works.

nuclio technical [**presentation in slideshare**](https://www.slideshare.net/iguazio/nuclio-overview-october-2017-80356865)
and a the video [**recording with demo**](https://www.youtube.com/watch?v=xlOp9BR5xcs)

**Note:** nuclio is still under development, and is not recommended for production use.

**In This Document**
- [Why Another "serverless" Project?](#why-another-serverless-project)
- [Getting-Started Start With nuclio](#getting-started-with-nuclio)
- [nuclio High-Level Architecture](#nuclio-high-level-architecture)
- [Function Examples](#nuclio-function-examples)
- [More Details and Links](#more-details-and-linksa)


## Why Another "serverless" Project?

We considered existing cloud and open-source serverless solutions, but none addressed our needs:

-  Real-time processing with minimal CPU and I/O overhead and maximum parallelism
-  Native integration with a large variety of data and event sources, and processing models

-  Abstraction of data resources from the function code, to support code portability, simplicity, and data-path acceleration
-  Simple debugging, regression testing, and multi-versioned CI/CD pipelines
-  Portability across low-power devices, laptops, on-prem clusters, and public clouds

We designed nuclio to be extendable, using a modular and layered approach.
We hope many will join us in developing new modules and integrations with more event and data sources, developer tools, and cloud platforms.

## Getting-Started Start With nuclio 

The simplest way to explore nuclio is to deploy the Playground micro-service (packaged as a kubernetes deployment), open a browser window, write code, deploy and execute functions. 
for more control use the nuctl CLI or APIs.
Head over to the [nuclio SDK repository](http://github.com/nuclio/nuclio-sdk) for a complete step-by-step guide for writing and deploying your first nuclio function.

![playground](docs/images/playground.png)

## nuclio High-Level Architecture

![architecture](docs/images/architecture.png)

<dl>
  <dt>Function Processors</dt>
  <dd>A processor listens on one or more event sources (for example, HTTP, Message Queue, Stream), and executes user functions with one or more parallel workers.
      The workers use language-specific runtimes to execute the function (via native calls, SHMEM, or shell).
      Processors use abstract interfaces to integrate with platform facilities for logging, monitoring, and configuration, allowing for greater portability and extensibility (such as logging to a screen, file, or log stream).
  </dd>
</dl>

<dl>
  <dt>Event Sources</dt>
  <dd>Functions can be invoked through a variety of event sources (such as HTTP, RabitMQ, Kafka, Kinesis, NATS, DynamoDB, iguazio v3io, or schedule), which are defined in the function specification.<br />
      Event sources are divided into several event classes (req/rep, async, stream, pooling), which define the sources' behavior.<br />
      Different event sources can plug seamlessly into the same function without sacrificing performance, allowing for portability, code reuse, and flexibility.
  </dd>
</dl>

<dl>
  <dt>Data Bindings</dt>
  <dd>Data-binding rules allow users to specify persistent input/output data resources to be used by the function.
      (Data connections are preserved between executions.)
      Bound data can be in the form of files, objects, records, messages etc.<br />
      The function specification may include an array of data-binding rules, each specifying the data resource and its credentials and usage parameters.<br />
      Data-binding abstraction allows using the same function with different data sources of the same type, and enables function portability.
  </dd>
</dl>

<dl>
  <dt>Playground</dt>
  <dd>The playground is a standalone container micro-service accessed through HTTP, it presents a code editor UI for editing, deploying and testing functions. This is the most user-friendly way to work with nuclio. The playground container comes with a version of the builder inside.
  </dd>
</dl>

<dl>
  <dt>nuctl cli</dt>
  <dd>nuctl is nuclio command line tool (cli), allowing users to list, create, build, update, execute and delete functions. 
  </dd>
</dl>

<dl>
  <dt>Controller</dt>
  <dd>A controller accepts function and event-source specifications, invokes builders and processors through an orchestration platform (such as Kubernetes), and manages function elasticity, life cycle, and versions.
  </dd>
</dl>

<dl>
  <dt>Builder</dt>
  <dd>A builder receives raw code and optional build instructions and dependencies, and generates the function artifact &mdash; a binary file or a Docker container image, which the builder can also push to a specified image repository.<br />
      The builder can run in the context of the CLI or as a separate service for automated development pipelines.
  </dd>
</dl>

<dl>
  <dt>Dealer</dt>
  <dd>A dealer is used with streaming and batch jobs to distribute a set of tasks or data partitions/shards among the available function instances, and guarantee that all tasks are completed successfully.
      For example, if a function reads from a message stream with 20 partitions, the dealer will guarantee that the partitions are distributed evenly across workers, taking into account the number of function instances and failures.
  </dd>
</dl>

<dl>
  <dt>nuclio SDK</dt>
  <dd>The nuclio SDK is used by function developers to write, test, and submit their code, without the need for the entire nuclio source tree.
  </dd>
</dl>

For more information about the nuclio architecture, see [nuclio Architecture](docs/architecture.md).

## nuclio Function Examples

The function demonstrated below, uses the `Event` and `Context` interfaces to handle inputs and logs, and returns a structured HTTP response (can also use a simple string as returned value).

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

## More Details and Links 

- [nuclio SDK repository](http://github.com/nuclio/nuclio-sdk) - step-by-step guide for writing and deploying nuclio functions.
- [nuctl CLI Guide](docs/nuctl/nuctl.md)
- [nuclio Architecture Details](docs/architecture.md)
- [nuclio Function Configuration and Metadata](docs/function-spec.md)
- [Using kubernetes ingress resources (as nuclio API gateway)](https://kubernetes.io/docs/concepts/services-networking/ingress/)

for more questions and help use nuclio [slack channel](https://nuclio-io.slack.com)