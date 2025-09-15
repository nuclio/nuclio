# Streaming HTTP Responses

>**⚠️ Warning:** Technical preview

Nuclio supports streaming HTTP responses for functions written in **Go** and **Python** runtimes. This feature is available **only for HTTP triggers** and is not supported for other trigger types.

Streaming allows a function to send data to the client in chunks as it becomes available, rather than waiting for the entire response to be ready.

## Supported Runtimes

- Go
- Python

## Supported Triggers

- HTTP

---

## Usage Examples

### Go Runtime

To stream a response in Go, use the `nuclio.NewResponseStream` object.

Data can be sent in two ways:
- `SendChunk`: Write a chunk of data (`[]byte`) directly to the stream.
- `StreamFrom`: Stream data from any `io.Reader` (useful for external sources).

**Always** use `defer responseStream.StopStreaming()` to properly close the stream.

By default, `NewResponseStream` uses an `io.Pipe` internally to connect the function's output to the HTTP response.
For advanced scenarios, a custom reader/writer can be provided using `NewCustomResponseStream`:

```go
responseStream := nuclio.NewCustomResponseStream(
    "text/plain",
    headers,
    200,
    customReader, // io.ReadCloser
    customWriter, // io.Writer
)
```

Example:

```go
package main

import (
    "fmt"
    "strings"

   "github.com/nuclio/nuclio-sdk-go"
)

func Streamer(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
    // Create a new response stream with "text/plain" content type and status code 200
    responseStream := nuclio.NewResponseStream(
        "text/plain",
        map[string]interface{}{
            "my-custom-header": "custom-value",
        },
        200,
    )
    // Create a channel to communicate when streaming starts or fails
    // Optional, but useful for catching errors
    startStreamErr := make(chan error)

    // Start streaming in a separate goroutine
    go func() {
        // Ensure stream is properly closed at the end of the goroutine
        defer responseStream.StopStreaming()

        // Notify the main thread that streaming has started successfully
        startStreamErr <- nil

        // Stream data chunk by chunk
        for ch := 'A'; ch <= 'Z'; ch++ {
            chunk := fmt.Sprintf("%c", ch)
            responseStream.SendChunk([]byte(chunk))
        }
    }()
    // Wait for the goroutine to report an error or success before returning the response
    err := <-startStreamErr
    return responseStream, err
}
```

### Python Runtime

In Python, streaming responses can be achieved by returning a generator (sync or async) as the response body, or by using `context.Response` with a generator.

#### Asynchronous generator example

```python
import aiofile

file_path = "/tmp/stream_outputter_lines.txt"

# yielding chunks asynchronously
async def stream_file_lines_async_handler(context, event):
    # Stream the file line by line asynchronously
    async with aiofile.AIOFile(file_path, "r") as afp:
        async for line in aiofile.LineReader(afp):
            yield line

# returning a context.Response with an async generator as the body
async def stream_file_lines_as_response_async_handler(context, event):
    return context.Response(body=stream_file_lines_async_handler(context, event))
```

#### Synchronous generator example

```python
# yielding chunks
def stream_file_lines_sync_handler(context, event):
    with open(file_path, "r") as f:
        for line in f:
            yield line

# returning a context.Response with an async generator as the body
async def stream_file_lines_as_response_sync_handler(context, event):
    return context.Response(body=stream_file_lines_sync_handler(context, event))
```
