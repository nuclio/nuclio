# AGENTS.md

This file provides guidance to AI Agents when working with code in this repository.

Nuclio is a high-performance serverless framework for real-time data/event processing. It builds user "function handlers" into container images and runs them on Kubernetes (or local Docker), feeding them events from a variety of triggers.

## Build & Test Commands

The Go module is `github.com/nuclio/nuclio` (Go toolchain pinned in `go.mod`). Most build/test targets shell out to Docker, since Nuclio compiles binaries and bakes them into images.

```sh
# Unit tests (the common loop). -race, -short, build tag test_unit:
make test-unit

# Run a single package's unit tests directly:
go test -tags=test_unit -race -short ./pkg/processor/worker/...

# Run one test by name:
go test -tags=test_unit -race -run TestMyThing ./pkg/processor/worker/...

# Lint (golangci-lint v2.x, auto-installed into .bin/) + fmt:
make lint
make fmt

# Coverage HTML:
make test-coverage

# Integration/e2e suites run inside the test container (require Docker, sometimes a cluster):
make test                 # dockerized integration (test_integration,test_local)
make test-docker-nuctl    # nuctl against local Docker platform
make test-k8s             # k8s integration suites
make govulncheck          # vulnerability scan over cmd/... pkg/...
```

Building images/binaries: `make build` (all images + nuctl), or individual targets `make processor`, `make controller`, `make dashboard`, `make autoscaler`, `make dlx`, `make nuctl`. `GOPATH` must be set for most targets (`ensure-gopath` guards this).

### Test build-tag convention (enforced)

**Every `*_test.go` file MUST start with a `//go:build test_<x>` build constraint.** `make lint` runs `ensure-test-files-annotated` and fails the build otherwise. Choose the tag by scope:

- `test_unit` — pure unit tests (no Docker/cluster).
- `test_integration` + one of `test_local` / `test_kube` / `test_iguazio` — integration tests needing Docker or a cluster.
- `test_functional`, `test_benchmark`, `test_broken` — special suites.

A test that lacks the right tag simply won't be compiled/run by the corresponding target, so match the tag to how the test is meant to run.

## Architecture

The end-to-end flow (see `docs/concepts/architecture.md`) is: **build → push to registry → deploy as CRD → controller reconciles → processor runs**.

### Binaries (`cmd/`)
- **processor** — the entrypoint baked into every function image. Reads function config, runs triggers, dispatches events to workers/runtimes, handles logs/metrics/health. This is the runtime hot path.
- **dashboard** — REST API + web UI (`pkg/dashboard/ui`); the primary control-plane API. Also embeds the function builder, so it deploys to either Docker (local) or Kubernetes.
- **controller** — Kubernetes operator that watches the `NuclioFunction`/`NuclioProject`/`NuclioAPIGateway`/`NuclioFunctionEvent` CRDs and reconciles native resources (Deployment, Service, Ingress, HPA…).
- **autoscaler** & **dlx** — scale-to-zero machinery. The autoscaler scales idle functions down; the **dlx** (Demand Layer Proxy) holds incoming requests and scales a function back **up from zero**.
- **nuctl** — the CLI (`pkg/nuctl/command` cobra commands, `pkg/nuctl/client`).

### Platform abstraction (`pkg/platform`)
`platform.Platform` is the central interface for all control-plane operations (deploy/get/delete functions, projects, api-gateways, invoke, etc.). Implementations:
- `pkg/platform/abstract` — shared base logic (`abstract.Platform`) embedded by both concrete platforms. **Most cross-cutting logic lives here**, not in the concrete platforms.
- `pkg/platform/kube` — Kubernetes platform; creates CRDs rather than native resources directly. CRD clients/operators in `pkg/platform/kube/{apis,clients,controller,functionres,apigatewayres,operator,monitoring,resourcescaler}`.
- `pkg/platform/local` — Docker-only platform (functions as plain containers).

`pkg/platform/factory` picks the implementation from `platformconfig`. When changing behavior, prefer editing `abstract` so both platforms inherit it.

### Function build (`pkg/processor/build`)
`builder.go` orchestrates turning a handler + config into an image: resolves the runtime, generates a Dockerfile, and delegates the actual image build/push to a `containerimagebuilderpusher` implementation:
- `pkg/containerimagebuilderpusher/docker.go` — local Docker daemon.
- `pkg/containerimagebuilderpusher/kaniko.go` — in-cluster, non-daemon builds (default on k8s).

Per-language build logic (base images, onbuild handling, dependency install) lives in `pkg/processor/build/runtime/<lang>`. Generated "onbuild" handler images come from the `handler-builder-*-onbuild` Makefile targets.

### Processor internals (`pkg/processor`)
Event path: **trigger → worker → runtime → handler**, with an `EventProcessor` abstraction at both the trigger and runtime levels (synchronous FIFO or async modes).
- `trigger/` — one subpackage per event source: `http`, `kafka`, `kinesis`, `mqtt`, `nats`, `rabbitmq`, `cron`, `pubsub`, `v3iostream`, plus shared scaffolding (`partitioned`, `poller`, `batcher`). Triggers self-register via `init()` into `trigger/registry.go`; `factory.go` instantiates from config.
- `runtime/` — per-language runtimes. `golang` is **native** (compiled in). `python`, `java`, `nodejs`, `dotnetcore`, `ruby` are **RPC/SHMEM** runtimes: the processor talks to an out-of-process language wrapper over a socket. Shared RPC machinery is in `runtime/rpc` (connection, encoder, result, control-message broker). `shell` runs executables. Runtimes register into `runtime/registry.go`.
- `worker/` — worker pool allocation; a blocking pool allocator enforces one-event-per-worker in sync mode.
- `controlcommunication/` — out-of-band control channel between processor and language wrappers (e.g. draining in-flight deliveries on close).

The **registry + factory pattern** is used throughout (triggers, runtimes, platforms, dashboard resources, metric/logger sinks): packages register themselves in `init()`, and a factory builds instances from config. To add a trigger/runtime, create the subpackage and register it; blank-import it where the binary wires its dependencies.

### Config types
- `pkg/functionconfig` — the function spec/config (the heart of the `NuclioFunction` CRD payload).
- `pkg/platformconfig` — platform-wide config (registry, logging, scaling defaults).
- `pkg/platform/types.go` — request/option structs for platform operations.

CRD Go types live under `pkg/platform/kube/apis`; regenerate with `make generate-crds` after changing them.

## Conventions
- Errors use `github.com/nuclio/errors` (wrap with `errors.Wrap`/`errors.New`), logging uses `github.com/nuclio/logger`. Follow the surrounding style rather than stdlib `errors`/`fmt.Errorf`. Always log with context if available.
- Enabled linters (`.golangci.yml`): `errcheck`, `gocritic`, `govet`, `ineffassign`, `misspell`, `revive`, `staticcheck`, `unconvert`, `unused`. Run `make lint` before assuming a change is complete.
- Non-Go runtime wrappers are tested separately: `make test-python`, `make test-nodejs`.
- JetBrains run configurations for most suites are checked in under `.run/`, prefixed by the build tag they use (e.g. `(test_unit)`, `(test_integ)`, `(test_kube)`).
