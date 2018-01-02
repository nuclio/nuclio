# Examples

To help you make the most of nuclio, the following function examples are provided:

## Go examples

- [Hello World](golang/helloworld) (`helloworld`): A simple function that showcases unstructured logging and a structured response.
- [Compliance Checker](golang/regexscan) (`regexscan`): A function that uses regular expressions to find patterns of social-security numbers (SSN), credit-card numbers, etc., using text input.
- [Image Resize and Convert](golang/image) (`image`): A function that demonstrates how to pass a binary-large object (blob) in an HTTP request body and response. The function defines an HTTP request that accepts a binary image or URL as input, converts the input to the target format and size, and returns the converted image in the HTTP response.
- [HTTP ingress](golang/ingress) (`ingress`): A simple function with an HTTP ingress configuration (using embedded YAML code) that routes specific URL paths to the function.
- [RabbitMQ](golang/rabbitmq) (`rabbitmq`): A multi-trigger function with a configuration that connects to RabbitMQ to read messages and write them to local ephemeral storage. If triggered with an HTTP `GET` request, the function returns the messages that it read from RabbitMQ.

## Python examples

- [Hello World](python/helloworld) (`helloworld`): A simple function that showcases unstructured logging and a structured response.
- [Encrypt](python/encrypt) (`encrypt`): A function that uses a third-party Python package to encrypt the event body, and showcases build commands for installing both OS-level and Python packages.
- [Face Recognizer](python/facerecognizer) (`face`): A function that uses Microsoft's face API, configured with function environment variables. The function uses third-party Python packages, which are installed by using an inline configuration.
- [Sentiment Analysis](python/sentiments) (`sentiments`): A function that uses the [vaderSentiment](https://github.com/cjhutto/vaderSentiment) library to classify text strings into a negative or positive sentiment score.
- [TensorFlow](python/tensorflow) (`tensorflow`): A function that uses the inception model of the [TensorFlow](https://www.tensorflow.org/) open-source machine-learning library to classify images. The function demonstrates advanced uses of nuclio with a custom base image, third-party Python packages, pre-loading data into function memory (the AI Model), structured logging, and exception handling.

## Shell examples

- [Image convert](shell/img-convert) (`img-convert`): A wrapper script around ImageMagick's "convert" executable, capable of generating thumbnails from received images (among other things). 

## NodeJS examples

- [Reverser](nodejs/reverser) (`reverser`): Returns the reverse of the body received in the event.
- [Dates](nodejs/dates) (`dates`): Uses moment.js (installed as part of the build) to add a given amount of time to "now", and returns this as string.
