# Deploying Pre-Built Functions

This guide goes through building functions to container images and then deploying them in a separate process.

#### In this document
- [Motivation](#motivation)
- [Building a function](#building-a-function)
- [Deploying the pre built function](#deploying-the-pre-built-function)
- [See also](#see-also)

## Motivation

If you followed the [function-deployment guide](/docs/tasks/deploying-functions.md), you built and deployed a function in a single convenient step using the `nuctl` CLI. However, it is sometimes desirable to build a function once and deploy it many times with different configuration. This guide will walk you through that process using `nuctl`.

In this scenario, you'll use the [Go hello-world example](/hack/examples/golang/helloworld) example.

## Building a function

Using `nuctl`, you can issue a build - specifying the URL of the Go hello-world:

```sh
nuctl build hello-world --path https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go \
    --registry $(minikube ip):5000
```

This produces the `nuclio/processor-hello-world:latest` image and pushes it to `192.168.64.8:5000`. The image contains everything the function needs to run, except a configuration file. 

## Deploying the pre-built function

To deploy the function to your platform, you'll use the `nuctl` deploy command with the `--run-image` option. When `--run-image` is present, `nuctl` does not initiate a build process - only creates a function in the platform and waits for it to become ready.

```sh
nuctl deploy hello-world --run-image localhost:5000/nuclio/processor-hello-world:latest \
    --runtime golang \
    --handler main:Handler \
    --namespace nuclio
```

You can deploy this function several times, providing different labels, triggers, etc. - yet still use the same image.

## See also

- [Deploying Functions](/docs/tasks/deploying-functions.md)

