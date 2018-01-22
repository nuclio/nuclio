# Roadmap

The day-to-day development is managed in the [GitHub issues](https://github.com/nuclio/nuclio/issues), but the following should serve as a high-level overview of the current nuclio features and future development plans.

## Current features

- Triggers
    - HTTP
    - NATS
    - Kafka
    - Kinesis
    - RabbitMQ
    - iguazio v3io
    - Azure Event Hub
    - cron (locally invoked)
- Platform abstraction
    - Kubernetes & Minikube
    - Google Kubernetes Engine (GKE)
    - Azure Container Service (AKS)
    - Local Docker
- Runtimes
    - Go (built as a plugin)
    - Python 2.7, 3.6 (CPython)
    - PyPy
    - Shell (invoke binary or script via exec)
    - V8 (Javascript and NodeJS)
- Data bindings
    - iguazio v3io
    - Azure Event Hub
- Configurable ingress for HTTP triggers
- Playground (ephemeral)
- Prometheus integration via push
- Command-line utility (`nuctl`), distributed under Github releases
- Versioning of artifacts
- Helm charts

## Under development

- Dealer (stream partition and scale orchestration)
- Playground revamp (UX, UI, and back end)
- Java runtime
- Raspberry Pi
- End-to-end testing automation

## In design

- Builder as seperate entity (currently integrated into in CLI and Playground) 
- Generic data bindings with multiple back ends (such as S3, Volumes, Streams, and K/V APIs)
- Zero scale on idle (currently functions scale starts with 1 pod)

## Coming up

- Function versioning and aliasing
- More linting
- Improve CPython performance
- Timeout enforcement
