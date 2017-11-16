# Examples

## Golang
1. [hello world](golang/helloworld): A simple function showcasing unstructured logging and a structured response
2. [Compliance checker](golang/regexscan): use Regex to find patterns of SSN, Credit card numbers, etc. in text input 
2. [image resize and convert](golang/image): Demonstrate the use of binary/blob in body and resp, accept an binary image or URL as input, convert to destination format and size, and return the converted image in the HTTP response
2. [HTTP ingress](golang/ingress): Demonstrate a simple function with http ingress configuration (in embedded YAML) to route specific URL paths to the function  
3. [rabbitmq](golang/rabbitmq): Configured to connect to RabbitMQ to read messages and write them to local ephemeral storage. If triggered with HTTP GET, returns the messages it read from RabbitMQ (multi trigger function)

## Python
1. [hello world](python/helloworld): A simple function showcasing unstructured logging and a structured response
2. [encrypt](python/encrypt): Uses a 3rd party Python package to encrypt the event body. Showcases build commands to install both OS level packages and Python packages
3. [face recognizer](python/facerecognizer): Uses Microsoft's face API configured by function environment variables. Uses 3rd party Python packages installed through inline configuration
4. [sentiment analysis](python/sentiments): Use [vaderSentiment](https://github.com/cjhutto/vaderSentiment) lib to classify text strings to neg/pos sentiment score 
