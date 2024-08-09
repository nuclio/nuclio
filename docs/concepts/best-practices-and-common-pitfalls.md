# Best Practices and Common Pitfalls

This guide aims to provide you with best practices for working with Nuclio and help you avoid common development pitfalls.

#### In this document

- [Use `init_context` instead of global variable declarations or function calls](#init_context-instead-of-global-context)
- [Use HTTP clients over browsers for HTTP(s) tests](#http-clients-for-testing)
- [Tweak worker configurations to resolve unavailable-server errors](#tweak-worker-cfg-to-resolve-http-503-errors)
- [Install CA certificates for alpine with HTTPS](#ca-certificates-for-alpine-w-https)

<a id="init_context-instead-of-global-context"></a>
## Use `init_context` instead of global variable declarations or function calls

When a Nuclio function is deployed, a runtime is created per worker. A "runtime" can be a Python interpreter, a Java JVM, a Go goroutine, etc., and it serves as the function's execution context.
Standard multithreading concepts apply also to Nuclio runtimes:

- Don't share data across workers (your "threads"), to prevent needless locking.
- Use thread-local storage (TLS) to maintain state rather than global variables.

For example, assume you need to create a connection to a database and maintain this connection throughout the lifetime of the function, to prevent creating a connection per request. One approach would be to store the database-connection object in a global variable, as demonstrated in the following code:
```python
my_db_connection = my_db.create_connection()

def handler(context, event):
    my_db_connection.query(...)
```
But there are two main issues with this approach:

1. Global variables might, at some point, be shared across workers (as is always the case in Go). This might cause race conditions in the connection while accessing the database.
2. If your connection fails, you'll be scratching your head trying to understand why your function fails on import.

The correct way to achieve your goal with Nuclio is by creating the database connection from the `init_context` function, as demonstrated in the following code:
```python
def handler(context, event):
    context.user_data.my_db_connection.query(...)


def init_context(context):

    # Create the DB connection under "context.user_data"
    setattr(context.user_data, 'my_db_connection', my_db.create_connection())
```

Because each Nuclio worker receives its own context across all runtimes, `init_context` is called per worker, passing the worker's specific context. You can also use variables such as `context.worker_id` and `context.trigger_name` if you need to uniquely identify the context.

<a id="http-clients-for-testing"></a>
## Use HTTP clients over browsers for HTTP(s) tests

When testing functions over HTTP(s), prefer an HTTP client (such as [curl](https://curl.se/), [HTTPie](https://httpie.org/), or [Postman](https://www.postman.com/)) over a browser (such as Google Chrome).
Browsers tend to create requests for **favicon.ico** and other sneaky things before you even press `ENTER`. This might cause confusion while debugging functions, as you can't really control when your function is invoked.

<a id="tweak-worker-cfg-to-resolve-http-503-errors"></a>
## Tweak worker configurations to resolve unavailable-server errors

When you issue an HTTP request to a Nuclio function, your HTTP client first creates a TCP connection to the function. Nuclio allows for a maximum of 256K concurrent connections, regardless of the configured number of workers, which should typically be sufficient to establish the TCP connection. When an event arrives over the connection, Nuclio first allocates a worker for the event. If a worker is available, it's assigned to handle the request, but if no work is available, one of the following happens:

1. If the worker-availability timeout is zero, a 503 ("Service Unavailable") HTTP error is returned immediately.
2. If the worker-availability timeout is a non-zero value, the connection enters a FIFO holding pattern until a worker is available or the timeout period elapses:
    1. If a worker is available before the end of the timeout, the event is handled normally.
    2. If no worker is available before the end of the timeout, a 503 ("Service Unavailable") HTTP error is returned at the end of the timeout.

To prevent such 503 errors, tweak the values of your function's number of workers and worker-availability timeout configurations, as necessary. The trade-off for having too many workers is higher memory consumption, and the optimal configuration highly depends on the amount of memory that your function code consumes.

<a id="ca-certificates-for-alpine-w-https"></a>
## Install CA certificates for alpine with HTTPS

Nuclio tries to default to the smallest image possible (except for Python, which currently defaults to a hefty python:3.9 base image). Therefore, where possible, the base image for your functions will be the [alpine](https://hub.docker.com/_/alpine) Docker image. To use HTTPS in alpine, you must include the following code in the build commands to install root CA certificates:
```sh
apk --update --nocache add ca-certificates
```

