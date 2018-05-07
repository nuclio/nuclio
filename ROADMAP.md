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
    - .NET core
    - PyPy
    - Shell (invoke binary or script via exec)
    - V8 (JavaScript and NodeJS)
    - Java (Jar and source)
- Data bindings
    - iguazio v3io
    - Azure Event Hub
- Configurable ingress for HTTP triggers
- HTTP REST API
- Dashboard (UI)
- Prometheus integration via push
- Microsoft Azure Application Insights integration for metrics and logging
- Command-line utility (`nuctl`), distributed under GitHub releases
- Versioning of artifacts
- Helm charts

## Under development

- Dealer (stream partition and scale orchestration)
- Scale out integration testing
- End-to-end testing automation
- Dark site support (no internet access), including support for user provided images
- VSCode plugin

## Backlog

- Function versioning and aliasing
- Builder as separate entity (currently integrated into in CLI and playground) 
- Timeout enforcement
- Zero scale on idle (currently functions scale starts with 1 pod)
- Generic data bindings with multiple back ends (such as S3, Volumes, Streams, and K/V APIs)
- Raspberry Pi
