# Best Practices and Common Pitfalls


## Don't declare global variables or call functions in the global context - use `init_context()` instead
When your Nuclio function is deployed, a "runtime" is created per worker. A runtime can be a Python interpreter, a Java JVM, a Go goroutine, etc. This is your context of execution and normal multithreading concepts should apply here: don't share data across workers (your "threads") to prevent needless locking and use "thread local storage" to maintain state rather than global variables. 

For example, let's say we need to create a connection to some database and maintain this connection throughout the lifetime of the function to prevent creating a connection per request. One option would be:

```
my_db_connection = my_db.create_connection()

def handler(context, event):
    my_db_connection.query(...)
```

There are two issues here:
1. Global variables may, at some point, be shared across workers (they already are in Go). This may cause race conditions in the connection while accessing the database
2. If your connection fails, you'll be scratching your head why your function fails on import

The correct way to do this in Nuclio is: 

```
def handler(context, event):
    context.user_data.my_db_connection.query(...)


def init_context(context):

    # create the connection under "context.user_data"
    setattr(context.user_data, 'my_db_connection', my_db.create_connection())
```

Since each Nuclio worker receives its own context across all runtimes, `init_context` is called per worker passing that specific worker's context. You can also use variables like `context.worker_id` and `context.trigger_name` if you need to uniquely identify the context.


## Prefer HTTP testing clients like `curl`, `httpie`, `Postman` over browsers (e.g. `Chrome`) when testing functions over HTTP/S

Browsers will tend to create requests to `favicon.ico` and other sneaky things before you even press enter. This may be cause confusion while debugging functions - as you can't really control when your function is invoked.


## Tweak number of workers or worker allocation timeout if you're seeing unexpected HTTP 503 responses

When you perform an HTTP reqeust towards the Nuclio function, your HTTP client will first create a TCP connection to the function. Nuclio accepts 256K concurrent connections regardless of the number of workers you configure so most likely the TCP connection will be established. When the event arrives over the connection, the first thing Nuclio does is allocate a worker for it. If a worker is available, it will be assigned to handle the request but if one is not available, one of three things will happen:

1. If worker availability timeout is 0, a `503 SERVICE UNAVAILABLE` HTTP error is returned immediately
2. If worker availability timeout is X, the specific connection will enter a FIFO holding pattern until a worker is available:
2.1. If a worker is available before X time passes, the event will be handled normally
2.2. If a worker is not available before X time passes, a `503 SERVICE UNAVAILABLE` HTTP error is returned immediately

To prevent this - tweak your number of workers and worker availability timeout accordingly. The tradeoff of having too many workers is memory consumption and highly depends on how much memory your function code consumes


## Make sure to install `ca-certificates` if you're using the alpine base image and plan to use HTTPS

Nuclio tries to default to the smallest image possible (except for Python which recently defaults to a hefty python:3.6 base). As such, where possible the base image for your functions will be alpine. To use HTTPS in alpine you must `apk --update --nocache add ca-certificates` in the build commands to install root certs. 