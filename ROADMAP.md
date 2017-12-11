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
- Versioning of artifacts
- Go functions compiled as plugins (much faster compilation, dependency support)
- Shell runtime
- Binary distributions of `nuctl`

## Under development

- Dealer
- Raspberry Pi
- Faster Python runtime (use PyPy as a DLL)
- End-to-end testing automation
- Playground revamp (UX, UI, and back end)
- V8 runtime (NodeJS)

## In design

- Scheduled invocation of functions
- Builder
- Generic data bindings with multiple back ends (such as S3, Volumes, Streams, and K/V APIs)

## Coming up

- More linting
- Auto scaling
- Timeout enforcement

