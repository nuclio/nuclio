# Roadmap

The day-to-day development is managed in the [GitHub issues](https://github.com/nuclio/nuclio/issues), but the following should serve as a high-level overview of the current nuclio features and future development plans.

## Current features

- Triggers
    - HTTP
    - NATS
    - Kafka
    - Kinesis
    - RabbitMQ
- Platform abstraction
    - Kubernetes & Minikube
    - Google Kubernetes Engine (GKE) 
    - Local
- Runtimes
    - Go
    - Python (basic)
- Data bindings (currently limited to iguazio APIs) 
- Configurable ingress for HTTP triggers
- Playground (ephemeral)
- Prometheus integration via push
- Command-line utility (`nuctl`)

## Under development

- Versioning of artifacts
- Dealer
- Raspberry Pi
- Go functions compiled as plugins (much faster compilation, dependency support)
- Shell runtime
- Faster Python runtime (use PyPy as a DLL)
- End-to-end testing automation

## In design

- Scheduled invocation of functions
- Builder
- Playground revamp (UX, UI, and back end)
- Generic data bindings with multiple back ends (such as S3, Volumes, Streams, and K/V APIs)

## Coming up

- More linting
- V8 runtime (NodeJS)
- Auto scaling
- Binary distributions of `nuctl`
- Timeout enforcement

