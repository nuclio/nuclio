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

---

## File Streaming via Headers (Legacy Workaround)

> **Note:** This feature was implemented as a workaround before native streaming support existed. For **Python** and **Go** runtimes, it's recommended to use the native streaming API described above. However, this header-based approach remains useful for other runtimes that don't yet support native streaming.

Nuclio supports streaming files via special response headers. This allows functions to stream file contents directly to the HTTP client without loading the entire file into memory.

### How It Works

When a function returns a response with specific headers, the HTTP trigger intercepts these headers and streams the file content directly:

1. **`X-nuclio-filestream-path`**: Specifies the file path to stream
2. **`X-nuclio-filestream-delete-after-send`** (optional): If set to **any** value, the file is automatically deleted after streaming completes

The trigger opens the file and streams it directly to the HTTP response using `io.ReadCloser`, avoiding memory buffering for large files.

### Example Usage

#### Go Runtime

```go
package main

import (
	"github.com/nuclio/nuclio-sdk-go"
)

func FileStreamer(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
	headers := map[string]interface{}{
		"X-nuclio-filestream-path": "/path/to/file.txt",
		// Optionally delete file after streaming
		"X-nuclio-filestream-delete-after-send": "true",
	}

	return nuclio.Response{
		Headers: headers,
		StatusCode: 200,
	}, nil
}
```

#### Python Runtime (Legacy - Use Native Streaming Instead)

```python
def file_streamer_handler(context, event):
    return context.Response(
        headers={
            "X-nuclio-filestream-path": "/path/to/file.txt",
            "X-nuclio-filestream-delete-after-send": "true"  # optional
        },
        status_code=200
    )
```

> **Recommendation for Python:** Use native streaming with generators (see examples above) instead of file streaming headers for better performance and cleaner code.

### When to Use

- **Use native streaming** (recommended): For Python and Go runtimes when you need to stream data
- **Use file streaming headers**: For other runtimes (Node.js, Java, .NET, Ruby, Shell) that don't yet support native streaming, or when you specifically need to stream files from the filesystem
