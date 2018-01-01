# Roadmap

The day-to-day development is managed in the [GitHub issues](https://github.com/nuclio/nuclio/issues), but the following should serve as a high-level overview of the current nuclio features and future development plans.

## Current features

- Triggers
    - HTTP
    - NATS
    - Kafka
    - Kinesis
    - RabbitMQ
    - iguazio V3IO
- Platform abstraction
    - Kubernetes & Minikube
    - Google Kubernetes Engine (GKE) 
    - Local
- Runtimes
    - Go
    - Python 2.7, 3.6
    - PyPy (faster, invoked as DLL)
    - Shell (invoke binary or script via exec)
    - node.js
- Data bindings (currently limited to iguazio APIs) 
- Configurable ingress for HTTP triggers
- Playground (ephemeral)
- Prometheus integration via push
- Command-line utility (`nuctl`)
- Versioning of artifacts
- Go functions compiled as plugins (much faster compilation, dependency support)
- Binary distributions of `nuctl`

## Under development

- helm charts 
- Dealer
- Raspberry Pi
- End-to-end testing automation
- Playground revamp (UX, UI, and back end)
- Java runtime
- Azure Kubernetes Service (AKS) support
- Cron schedule trigger (local)

## In design

- Builder as seperate entity (currently used in CLI and Playground) 
- Azure Event-hub (stream) trigger 
- Generic data bindings with multiple back ends (such as S3, Volumes, Streams, and K/V APIs)
- Zero scale on idle (currently functions scale starts with 1 PODs)

## Coming up

- More linting
- Improve Cpython performance
- Timeout enforcement

