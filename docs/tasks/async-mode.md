# Asynchronous Mode Support in Nuclio

>**⚠️ Warning:** Technical preview

## Overview

Nuclio now supports asynchronous function invocation within a single worker process.
This feature enhances performance and resource utilization, particularly in use cases where functions handle multiple concurrent requests. It is especially beneficial for I/O-bound operations.
Currently, asynchronous mode is available only for HTTP triggers and the Python runtime.

## Enabling Async Mode

To enable asynchronous mode, set the `spec.triggers.trigger-name.mode` field to `async` in the function's configuration.
Asynchronous mode operates by establishing multiple socket connections between the Nuclio processor and the worker process. By default, Nuclio creates `1000` connections per worker.
This value can be customized using the `spec.triggers.trigger-name.async.maxConnectionsNumber` field.

## Architecture Considerations and Failure Recovery

Architecture description can be found [here](../concepts/architecture.md).

Async mode is designed to complement, not replace, Nuclio synchronous processing. It should be used only when asynchronous handling aligns with the function’s behavior.
If a blocking operation is executed within an async function, it can block the entire worker process, preventing other requests from being processed.

When `spec.eventTimeout` is configured, any connection that exceeds the timeout is marked for restart. The connection is then closed, and the Nuclio processor attempts to re-establish it.
If the connection cannot be re-established due to a blocking operation or any other problem, the processor retries up to 3 times, with a 30-second timeout for each attempt.
If all retries fail, the worker process is restarted to ensure proper recovery.

### Concurrency and Connection Management

The total number of events that can be processed concurrently is determined by the product of `numWorkers` and `maxConnectionsNumber`.
This represents the maximum number of events that can be processed concurrently by one pod.

If all connections are occupied and a new event arrives, the event will wait for a connection to become available for up to `spec.triggers.trigger-name.async.connectionAvailabilityTimeout`.
By default, this timeout is set to 10 seconds, but it can be customized to any desired value.

### Slow `init_context` and Startup Timeouts

When `init_context` takes a long time (e.g., loading a large model), the Go processor must wait for the Python wrapper to start listening before it can establish connections.
By default the startup budget is **3 × `readinessTimeoutSeconds`**, which scales automatically with the function's configured readiness window.

If your `init_context` routinely exceeds this budget, set an explicit override:

```yaml
spec:
  readinessTimeoutSeconds: 300   # 5 minutes
  triggers:
    myTrigger:
      kind: http
      mode: async
      async:
        establishConnectionTimeout: "20m"    # override: 20 minutes
```

`establishConnectionTimeout` accepts any value valid for Go's [`time.ParseDuration`](https://golang.org/pkg/time/#ParseDuration) (e.g., `"5m"`, `"600s"`).

### Troubleshooting

#### "dial tcp 127.0.0.1:1337: connect: connection refused"

This error occurs during deployment when the Python wrapper has not started listening by the time the startup budget is exhausted.

**Causes:**
- `init_context` takes longer than `3 × readinessTimeoutSeconds`

**Solutions:**
1. **Increase readiness timeout:** Set `spec.readinessTimeoutSeconds` to a value larger than your `init_context` duration — the startup budget will grow automatically (3×).
2. **Set an explicit override:** Set `spec.triggers.trigger-name.async.establishConnectionTimeout` to a fixed duration if you need finer control (e.g., `"10m"`).

#### "Failed to allocate connection for processing event"

This error occurs when a new event arrives but all connections are occupied, and the `connectionAvailabilityTimeout` expires before a connection becomes available.

**Causes:**
- High concurrency with long-running requests
- Blocking operations in async handlers
- `maxConnectionsNumber` is too low for the workload
- `connectionAvailabilityTimeout` is too short

**Solutions:**
1. **Increase timeout:** Set `spec.triggers.trigger-name.async.connectionAvailabilityTimeout` to a higher value (default is 10s)
2. **Increase connections:** Set `spec.triggers.trigger-name.async.maxConnectionsNumber` to a higher value (default is 1000)
3. **Scale horizontally:** Increase `numWorkers` or pod replicas
4. **Review handler code:** Ensure async handlers don't perform blocking operations