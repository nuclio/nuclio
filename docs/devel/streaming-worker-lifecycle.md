# HTTP Streaming Worker Lifecycle

This document describes the worker allocation, streaming, and release flows in the HTTP trigger, with special attention to edge cases like client disconnection and panic handling.

## Architecture Overview

### Two-Level Event Processor Hierarchy

There are **two levels** of event processors, each with their own allocator:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              HTTP Trigger                                    │
│                                                                             │
│                     ┌────────────────────────────────────┐                  │
│                     │         Worker Allocator           │                  │
│                     │  (managed by HTTP trigger)         │                  │
│                     │                                    │                  │
│                     │  Sync Mode:  BlockingPoolAllocator │                  │
│                     │  Async Mode: NonBlockingPoolAlloc  │                  │
│                     └──────────────┬─────────────────────┘                  │
└────────────────────────────────────┼────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Worker (EventProcessor) = Python Process                  │
│                                                                             │
│    Each worker wraps a Python process and manages connections to it         │
│                                                                             │
│                     ┌────────────────────────────────────┐                  │
│                     │       Connection Allocator         │                  │
│                     │  (managed by Python runtime)       │                  │
│                     │                                    │                  │
│                     │  1 conn:  NonBlockingSingleton     │                  │
│                     │  N conns: BlockingPoolAllocator    │                  │
│                     └──────────────┬─────────────────────┘                  │
└────────────────────────────────────┼────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Connection (EventProcessor) = Socket to Python            │
│                                                                             │
│    Each connection is a socket connection to the Python wrapper             │
│    Used to send events and receive responses (including stream chunks)      │
│                                                                             │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐         │
│  │ Python Handler  │───▶│   Generator     │───▶│  yield chunks   │         │
│  │                 │    │                 │    │  via io.Pipe    │         │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘         │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Allocator Types by Mode

| Level | Component | Sync Mode | Async Mode |
|-------|-----------|-----------|------------|
| Level 1 | **Worker** (Python process) | `BlockingPoolAllocator` | `NonBlockingPoolAllocator` |
| Level 2 | **Connection** (socket to Python) | `NonBlockingSingletonAllocator` (1 conn) | `BlockingPoolAllocator` (N conns) |

> **Note:** In sync mode, typically there's 1 connection per worker, so `NonBlockingSingletonAllocator` is used.
> In async mode, multiple connections can be pooled, so `BlockingPoolAllocator` is used.

**Blocking Pool Allocator:**
- Worker/connection is "locked" during use
- Must be explicitly released before another request can use it
- Used when request-response must be sequential

**Non-Blocking Pool Allocator:**
- Round-robin allocation
- Worker is not "locked" - multiple requests can use same worker
- Worker just routes to available connections
- Used in async mode for higher concurrency

### Request Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              HTTP Trigger                                    │
│                                                                             │
│  ┌─────────────┐    ┌──────────────────┐    ┌─────────────────────────┐    │
│  │   fasthttp  │───▶│  Request Handler │───▶│  SetBodyStreamWriter    │    │
│  │   Server    │    │                  │    │  (async callback)       │    │
│  └─────────────┘    └──────────────────┘    └─────────────────────────┘    │
│                              │                          │                   │
│                              ▼                          ▼                   │
│                     ┌────────────────┐         ┌────────────────┐          │
│                     │ WorkerAllocator│         │   io.Copy()    │          │
│                     │  (Level 1)     │         │  Stream Data   │          │
│                     └────────────────┘         └────────────────┘          │
└─────────────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│              Worker (Python Process) - Level 1 EventProcessor                │
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────┐     │
│  │                    Connection Allocator (Level 2)                  │     │
│  │                                                                    │     │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐               │     │
│  │  │ Connection 1│  │ Connection 2│  │ Connection N│               │     │
│  │  │  (socket)   │  │  (socket)   │  │  (socket)   │               │     │
│  │  └─────────────┘  └─────────────┘  └─────────────┘               │     │
│  └───────────────────────────────────────────────────────────────────┘     │
│                              │                                              │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐         │
│  │  ProcessEvent   │───▶│  StreamStart    │───▶│  ProcessStream  │         │
│  │ (allocates conn)│    │  (returns fast) │    │  (goroutine)    │         │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘         │
└─────────────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│              Connection (Socket) - Level 2 EventProcessor                    │
│                                                                             │
│  Communicates with Python wrapper via socket                                │
│  Handles streaming by waiting for chunks in ProcessStream                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Key Components

### 1. Worker Release Mechanism (sync.Once)

```go
var releaseOnce sync.Once
releaseWorker := func() {
    releaseOnce.Do(func() {
        h.WorkerAllocator.Release(workerInstance)
    })
}
```

- **Purpose**: Ensure worker is released exactly once
- **Why sync.Once**: Multiple code paths could trigger release (defer, callback, error handling)

### 2. Streaming Mode Flag

```go
streamingMode := false

defer func() {
    if !streamingMode {
        releaseWorker()
    }
}()

// In streaming case:
streamingMode = true
ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
    defer releaseWorker()
    // ... streaming logic
})
```

- **Purpose**: Prevent outer defer from releasing worker before streaming completes
- **Why needed**: `SetBodyStreamWriter` is non-blocking - callback runs AFTER handler returns

## Flow Diagrams

### Flow 1: Normal Non-Streaming Response

```
┌──────────────────────────────────────────────────────────────────┐
│                    Non-Streaming Request Flow                     │
└──────────────────────────────────────────────────────────────────┘

Client              HTTP Trigger           Worker          Python
  │                      │                    │               │
  │──── Request ────────▶│                    │               │
  │                      │─── Allocate ──────▶│               │
  │                      │                    │─── Process ──▶│
  │                      │                    │◀── Response ──│
  │                      │◀── Response ───────│               │
  │                      │                    │               │
  │                      │─── releaseWorker() │               │
  │                      │    (sync.Once) ───▶│ (back to pool)│
  │                      │                    │               │
  │◀─── Response ────────│                    │               │
  │                      │                    │               │
  │                      │─── defer runs ─────│               │
  │                      │    streamingMode=  │               │
  │                      │    false           │               │
  │                      │    releaseWorker() │               │
  │                      │    (no-op, already │               │
  │                      │     released)      │               │
```

### Flow 2: Successful Streaming Response

```
┌──────────────────────────────────────────────────────────────────┐
│                    Streaming Request Flow (Success)               │
└──────────────────────────────────────────────────────────────────┘

Client              HTTP Trigger           Worker          Python
  │                      │                    │               │
  │──── Request ────────▶│                    │               │
  │                      │─── Allocate ──────▶│               │
  │                      │                    │─── Process ──▶│
  │                      │                    │◀─ StreamStart─│
  │                      │◀── StreamStart ────│               │
  │                      │                    │               │
  │                      │ streamingMode=true │               │
  │                      │                    │               │
  │                      │ SetBodyStreamWriter│               │
  │                      │ (registers callback│               │
  │                      │  returns immediately)              │
  │                      │                    │               │
  │                      │─── Handler returns │               │
  │                      │    defer runs:     │               │
  │                      │    streamingMode=  │               │
  │                      │    true → SKIP     │               │
  │                      │                    │               │
  │                      │                    │               │
  │                   ┌──┴──────────────────────────────────┐ │
  │                   │  Callback runs (fasthttp writes)    │ │
  │                   └──┬──────────────────────────────────┘ │
  │                      │                    │               │
  │◀── Chunk 1 ──────────│◀─── io.Copy ───────│◀── yield ────│
  │◀── Chunk 2 ──────────│◀─── io.Copy ───────│◀── yield ────│
  │◀── Chunk 3 ──────────│◀─── io.Copy ───────│◀── yield ────│
  │                      │                    │               │
  │                      │◀─── io.Copy done ──│◀── EOF ──────│
  │                      │                    │               │
  │                      │ typedResponse.Close()              │
  │                      │                    │               │
  │                      │ StreamProcessedSuccessfully()      │
  │                      │                    │               │
  │                      │─── defer releaseWorker()           │
  │                      │    (in callback) ─▶│ (back to pool)│
```

### Flow 3: Client Disconnects Mid-Stream

```
┌──────────────────────────────────────────────────────────────────┐
│                Client Disconnection During Streaming              │
└──────────────────────────────────────────────────────────────────┘

Client              HTTP Trigger           Worker          Python
  │                      │                    │               │
  │──── Request ────────▶│                    │               │
  │                      │─── Allocate ──────▶│               │
  │                      │◀── StreamStart ────│◀─────────────│
  │                      │                    │               │
  │◀── Chunk 1 ──────────│◀─── io.Copy ───────│◀── yield ────│
  │◀── Chunk 2 ──────────│◀─── io.Copy ───────│◀── yield ────│
  │                      │                    │               │
  │ ╳ DISCONNECT ╳       │                    │               │
  │                      │                    │               │
  │                      │─── io.Copy FAILS ──│               │
  │                      │    (write error)   │               │
  │                      │                    │               │
  │                      │ Log: "Failed to    │               │
  │                      │  copy stream..."   │               │
  │                      │                    │               │
  │                      │ SetStatus(         │               │
  │                      │  RestartRequired)  │  ◀─────────── │ (sync mode only)
  │                      │                    │               │
  │                      │ typedResponse.Close()              │
  │                      │    (closes read    │               │
  │                      │     end of pipe)   │               │
  │                      │                    │               │
  │                      │                    │    ┌─────────────────┐
  │                      │                    │    │ Python tries    │
  │                      │                    │    │ to yield next   │
  │                      │                    │    │ → ErrClosedPipe │
  │                      │                    │    │ → generator     │
  │                      │                    │    │    stops        │
  │                      │                    │    └─────────────────┘
  │                      │                    │               │
  │                      │─── defer releaseWorker()           │
  │                      │                    │               │
  │                      │    SocketAllocator.Release():      │
  │                      │    sees RestartRequired            │
  │                      │    → triggers restart              │
```

### Flow 3b: Client Disconnects Mid-Stream (Async Mode)

In async mode, we use a **non-blocking worker allocator** but still use a **blocking connection allocator** for Python connections. The restart is handled differently:

```
┌──────────────────────────────────────────────────────────────────┐
│          Client Disconnection During Streaming (Async Mode)       │
└──────────────────────────────────────────────────────────────────┘

Client              HTTP Trigger           Worker          Python/Connection
  │                      │                    │               │
  │──── Request ────────▶│                    │               │
  │                      │─── Allocate ──────▶│               │
  │                      │◀── StreamStart ────│◀─────────────│
  │                      │                    │               │
  │◀── Chunk 1 ──────────│◀─── io.Copy ───────│◀── yield ────│
  │◀── Chunk 2 ──────────│◀─── io.Copy ───────│◀── yield ────│
  │                      │                    │               │
  │ ╳ DISCONNECT ╳       │                    │               │
  │                      │                    │               │
  │                      │─── io.Copy FAILS ──│               │
  │                      │    (write error)   │               │
  │                      │                    │               │
  │                      │ Log: "Failed to    │               │
  │                      │  copy stream..."   │               │
  │                      │                    │               │
  │                      │ (NO SetStatus -    │               │
  │                      │  async mode)       │               │
  │                      │                    │               │
  │                      │ typedResponse.Close()              │
  │                      │    ┌───────────────────────────────────────┐
  │                      │    │  Closes READ end of io.Pipe           │
  │                      │    │  Writer (Python) will get             │
  │                      │    │  ErrClosedPipe on next write          │
  │                      │    └───────────────────────────────────────┘
  │                      │                    │               │
  │                      │                    │               │
  │                      │                    │    ┌─────────────────────┐
  │                      │                    │    │ Python generator    │
  │                      │                    │    │ yields next chunk   │
  │                      │                    │    └──────────┬──────────┘
  │                      │                    │               │
  │                      │                    │               ▼
  │                      │                    │    ┌─────────────────────┐
  │                      │                    │    │ SendChunk() called  │
  │                      │                    │    │ in ProcessStream    │
  │                      │                    │    └──────────┬──────────┘
  │                      │                    │               │
  │                      │                    │               ▼
  │                      │                    │    ┌─────────────────────┐
  │                      │                    │    │ Write to closed     │
  │                      │                    │    │ pipe FAILS          │
  │                      │                    │    │ → ErrClosedPipe     │
  │                      │                    │    └──────────┬──────────┘
  │                      │                    │               │
  │                      │                    │               ▼
  │                      │                    │    ┌─────────────────────┐
  │                      │                    │    │ ProcessStream       │
  │                      │                    │    │ returns error       │
  │                      │                    │    │                     │
  │                      │                    │    │ defer sets:         │
  │                      │                    │    │ SetStatus(          │
  │                      │                    │    │  RestartRequired)   │
  │                      │                    │    └──────────┬──────────┘
  │                      │                    │               │
  │                      │                    │               ▼
  │                      │                    │    ┌─────────────────────┐
  │                      │                    │    │ Connection released │
  │                      │                    │    │ with RestartRequired│
  │                      │                    │    │ → allocator sees it │
  │                      │                    │    │ → triggers restart  │
  │                      │                    │    └─────────────────────┘
```

**Key difference from Sync Mode:**

| Aspect | Sync Mode | Async Mode |
|--------|-----------|------------|
| **Level 1: Worker Allocator** | `BlockingPoolAllocator` (workers locked during use) | `NonBlockingPoolAllocator` (round-robin) |
| **Level 2: Connection Allocator** | `NonBlockingSingletonAllocator` (1 conn per worker) | `BlockingPoolAllocator` (N conns pooled) |
| SetStatus in HTTP trigger | YES (before worker release) | NO (not needed) |
| Restart triggered by | HTTP trigger sets status on worker | ProcessStream sets status on connection |
| Why? | Worker reused immediately after release, need early marking | Worker not locked; connection not reused until ProcessStream completes |

**Why async mode doesn't need explicit SetStatus:**

1. In async mode, the **non-blocking worker allocator** uses round-robin - workers aren't "locked" during use
2. The **connection** is still managed by the blocking allocator in `ProcessStream`
3. `ProcessStream` runs in a goroutine and will eventually:
   - Try to `SendChunk()` to the closed pipe
   - Get `ErrClosedPipe`
   - Set `RestartRequired` in its defer
   - Release connection with restart status
4. The connection won't be reused until `ProcessStream` completes and releases it

### Flow 4: Panic Before Streaming Setup

```
┌──────────────────────────────────────────────────────────────────┐
│                Panic Before streamingMode=true                    │
└──────────────────────────────────────────────────────────────────┘

Client              HTTP Trigger           Worker
  │                      │                    │
  │──── Request ────────▶│                    │
  │                      │─── Allocate ──────▶│
  │                      │◀── Response ───────│
  │                      │                    │
  │                      │ streamingMode=false│
  │                      │                    │
  │                      │ PANIC! ────────────│
  │                      │                    │
  │                      │─── defer runs ─────│
  │                      │    streamingMode=  │
  │                      │    false           │
  │                      │    → releaseWorker()
  │                      │                   ▶│ (back to pool)
  │                      │                    │
  │                      │ HandleSubmitPanic  │
  │                      │ also catches panic │
  │                      │ (pointer to worker)│
```

### Flow 5: Panic After Streaming Setup

```
┌──────────────────────────────────────────────────────────────────┐
│                Panic After streamingMode=true                     │
└──────────────────────────────────────────────────────────────────┘

Client              HTTP Trigger           Worker
  │                      │                    │
  │──── Request ────────▶│                    │
  │                      │─── Allocate ──────▶│
  │                      │◀── StreamStart ────│
  │                      │                    │
  │                      │ streamingMode=true │
  │                      │                    │
  │                      │ SetBodyStreamWriter│
  │                      │ (callback registered)
  │                      │                    │
  │                      │ PANIC! ────────────│
  │                      │                    │
  │                      │─── defer runs ─────│
  │                      │    streamingMode=  │
  │                      │    true            │
  │                      │    → SKIP release  │
  │                      │                    │
  │                      │                    │
  │                   ┌──┴──────────────────────────────────┐
  │                   │  Callback may or may not run        │
  │                   │  depending on fasthttp behavior     │
  │                   └──┬──────────────────────────────────┘
  │                      │                    │
  │                      │ If callback runs:  │
  │                      │ defer releaseWorker()
  │                      │                   ▶│ (back to pool)
  │                      │                    │
  │                      │ If callback doesn't│
  │                      │ run: WORKER LEAK!  │
  │                      │ (edge case)        │
```

## Restart Propagation

When a streaming error occurs in sync mode:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Restart Status Propagation                    │
└─────────────────────────────────────────────────────────────────┘

  HTTP Trigger                    SocketAllocator              Runtime
       │                                │                         │
       │ workerInstance.SetStatus(      │                         │
       │   RestartRequired)             │                         │
       │                                │                         │
       │ releaseWorker() ──────────────▶│                         │
       │                                │                         │
       │                    if connection.GetStatus() ==          │
       │                       RestartRequired:                   │
       │                       sa.SetStatus(RestartRequired)      │
       │                                │                         │
       │                                │──── Restart triggered ─▶│
       │                                │                         │
       │                                │                    Python process
       │                                │                    restarted
```

## Key Design Decisions

### 1. Why sync.Once for release?
- Multiple code paths could trigger release
- Prevents double-release which would corrupt the worker pool

### 2. Why streamingMode flag?
- `SetBodyStreamWriter` is non-blocking
- Handler returns BEFORE callback executes
- Without flag, outer defer would release worker too early
- Results in worker being reused while still streaming → mixed responses

### 3. Why RestartRequired on disconnect (sync mode only)?

**Sync Mode:**
- Uses **blocking worker allocator** - workers are locked during use and reused immediately after release
- If we release without `RestartRequired`, next request might get a worker with corrupted connection state
- We must set `RestartRequired` BEFORE release to ensure allocator knows to restart

**Async Mode:**
- Uses **non-blocking worker allocator** - round-robin, workers not locked
- Connection is still managed by **blocking connection allocator** inside `ProcessStream` goroutine
- `ProcessStream` will eventually fail when trying to write to closed pipe:
  1. `typedResponse.Close()` closes read end of pipe
  2. Python generator tries to yield next chunk
  3. `SendChunk()` fails with `ErrClosedPipe`
  4. `ProcessStream` sets `RestartRequired` in its defer
  5. Connection released with restart status → allocator triggers restart
- No race condition because connection isn't released until `ProcessStream` completes

### 4. Why pointer to workerInstance in HandleSubmitPanic?
- Defer captures variable VALUE at registration time
- workerInstance is nil when defer is registered
- Pointer ensures we get current value at panic time

## Testing Considerations

When testing streaming:

1. **Parallel requests**: Verify workers aren't mixed up
2. **Client disconnect**: Verify cleanup happens
3. **Long-running generators**: Verify they stop on disconnect
4. **Panic scenarios**: Verify workers are properly released

## Related Code Locations

- HTTP Trigger: `pkg/processor/trigger/http/trigger.go`
- Worker Allocator: `pkg/processor/eventprocessor/pool.go`
- Socket Allocator: `pkg/processor/runtime/rpc/connection/socketallocator.go`
- Process Stream: `pkg/processor/runtime/rpc/connection/abstract.go`
- Response Stream: `github.com/nuclio/nuclio-sdk-go/response.go`

