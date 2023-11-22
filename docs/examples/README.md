# Examples

To help you make the most of Nuclio, the following function examples are provided:

Note: all function examples have the explicit field `disableDefaultHttpTrigger: false`, so they are deployable even if default http trigger creation is disabled on platform configuration level.
[Function configuration docs](../reference/function-configuration/function-configuration-reference.md)
## Go examples

- [Hello World](https://github.com/nuclio/nuclio/tree/development/hack/examples/golang/helloworld) (`helloworld`): A simple function that showcases unstructured logging and a structured response.
- [Compliance Checker](https://github.com/nuclio/nuclio/tree/development/hack/examples/golang/regexscan) (`regexscan`): A function that uses regular expressions to find patterns of social-security numbers (SSN), credit-card numbers, etc., using text input.
- [Image Resize and Convert](https://github.com/nuclio/nuclio/tree/development/hack/examples/golang/image) (`image`): A function that demonstrates how to pass a binary-large object (blob) in an HTTP request body and response. The function defines an HTTP request that accepts a binary image or URL as input, converts the input to the target format and size, and returns the converted image in the HTTP response.
- [HTTP Ingress](https://github.com/nuclio/nuclio/tree/development/hack/examples/golang/ingress) (`ingress`): A simple function with an HTTP ingress configuration (using embedded YAML code) that routes specific URL paths to the function.
- [RabbitMQ](https://github.com/nuclio/nuclio/tree/development/hack/examples/golang/rabbitmq) (`rabbitmq`): A multi-trigger function with a configuration that connects to RabbitMQ to read messages and write them to local ephemeral storage. If triggered with an HTTP `GET` request, the function returns the messages that it read from RabbitMQ.
- [Azure Event Hub](https://github.com/nuclio/nuclio/tree/development/hack/examples/golang/eventhub) (`eventhub`): An Azure Event Hub triggered function with a configuration that connects to an Azure Event Hub. The function reads messages from two partitions, process the messages, invokes another function, and sends the processed payload to another Azure Event Hub. You can find a full demo scenario [here](https://github.com/nuclio/demos/tree/master/fleet-alarm-detection-azure).
- [Call Function](https://github.com/nuclio/nuclio/tree/development/hack/examples/golang/callfunction) (`callfunction`): A set of two functions that demonstrates the `CallFunction` feature:

    - [`fibonacci`](https://github.com/nuclio/nuclio/tree/development/hack/examples/golang/callfunction/fibonacci/fibonacci.go) - For input parameter `n`, returns the `n`-th number in the Fibonacci series (`fib(n)`).
    - [`fibonaccisum`](https://github.com/nuclio/nuclio/tree/development/hack/examples/golang/callfunction/fibonaccisum/fibonaccisum.go) - Uses `CallFunction` to call the `fibonacci` function for input numbers 2, 10, and 17, and returns the sum of the three returned Fibonacci numbers (`fib(2)+fib(10)+fib(17)`).

    To try the example, deploy the `fibonacci` function and name it `fibonacci`; then, deploy the `fibonaccisum` function to the same namespace, using your preferred function name, and call it.

## Python examples

- [Hello World](https://github.com/nuclio/nuclio/tree/development/hack/examples/python/helloworld) (`helloworld`): A simple function that showcases unstructured logging and a structured response.
- [Encrypt](https://github.com/nuclio/nuclio/tree/development/hack/examples/python/encrypt) (`encrypt`): A function that uses a third-party Python package to encrypt the event body, and showcases build commands for installing both OS-level and Python packages.
- [Face Recognizer](https://github.com/nuclio/nuclio/tree/development/hack/examples/python/facerecognizer) (`face`): A function that uses Microsoft's face API, configured with function environment variables. The function uses third-party Python packages, which are installed by using an inline configuration.
- [Sentiment Analysis](https://github.com/nuclio/nuclio/tree/development/hack/examples/python/sentiments) (`sentiments`): A function that uses the [vaderSentiment](https://github.com/cjhutto/vaderSentiment) library to classify text strings into a negative or positive sentiment score.
- [TensorFlow](https://github.com/nuclio/nuclio/tree/development/hack/examples/python/tensorflow) (`tensorflow`): A function that uses the inception model of the [TensorFlow](https://www.tensorflow.org/) open-source machine-learning library to classify images. The function demonstrates advanced uses of Nuclio with a custom base image, third-party Python packages, pre-loading data into function memory (the AI Model), structured logging, and exception handling.

## Shell examples

- [Image Convert](https://github.com/nuclio/nuclio/tree/development/hack/examples/shell/img-convert) (`img-convert`): A wrapper script around ImageMagick's **convert** executable, which is capable of generating thumbnails from received images (among other things). 

## NodeJS examples

- [Reverser](https://github.com/nuclio/nuclio/tree/development/hack/examples/nodejs/reverser) (`reverser`): Returns the reverse of the body received in the event.
- [Dates](https://github.com/nuclio/nuclio/tree/development/hack/examples/nodejs/dates) (`dates`): Uses **moment.js** (which is installed as part of the build) to add a specified amount of time to `"now"`, and returns this amount as a string.

## .NET Core 7.0 examples

- [Reverser](https://github.com/nuclio/nuclio/tree/development/hack/examples/dotnetcore/reverser) (`reverser`): Returns the reverse of the body received in the event.
- [Hello World](https://github.com/nuclio/nuclio/tree/development/hack/examples/dotnetcore/helloworld):  (`helloworld`): A simple function that showcases structured logging, unstructured logging and a structured response.

## Java Examples

- [Empty](https://github.com/nuclio/nuclio/tree/development/hack/examples/java/empty) (`empty`): A simple function that returns an empty string.
- [Reverser](https://github.com/nuclio/nuclio/tree/development/hack/examples/java/reverser) (`reverser`): Returns the reverse of the body received in the event, also shows how to log.

