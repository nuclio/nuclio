# Roadmap

The day to day is managed in [Github issues](https://github.com/nuclio/nuclio/issues) but the below should serve as a high level overview of what nuclio has right now as opposed to what is planned.

## Current features
* Triggers
    * HTTP
    * NATS
    * Kafka
    * Kinesis
    * RabbitMQ
* Platform abstraction:
    * Kubernetes & minikube
    * Google GKE 
    * Local
* Runtimes:
    * Golang
    * Python (basic)
* Data bindings (currently limited to iguazio APIs) 
* Configurable ingress for HTTP triggers
* Playground (ephemeral)
* Prometheus integration via push
* Command line utility (nuctl)

## Under development
* Versioning of artifacts
* Dealer
* Raspberry Pi
* Golang functions compiled as plugins (much faster compilation, dependency support)
* Shell runtime
* Faster Python runtime (use PyPy as dll)
* End-to-end testing automation

## In design
* Scheduled invocation of functions
* Builder
* Playground revamp (UX, UI and backend)
* Generic data bindings with multiple back-end (e.g. S3, Volumes, Streams, K/V APIs)

## Coming up
* More linting
* V8 runtime (NodeJS)
* Autoscaling
* Binary distributions of nuctl
* Timeout enforcement
