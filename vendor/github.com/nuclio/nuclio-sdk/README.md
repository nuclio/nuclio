# nuclio-sdk
SDK for working with Nuclio

### Getting started
Get the stuff:
```
go get -d github.com/nuclio/nuclio-sdk
go get github.com/nuclio/nuclio/cmd/nuclio-build
```

Add the bin to path, so we can run nuclio-build:
```
PATH=$PATH:$GOPATH/bin
```
Build the example from SDK example and push it to your registry:
```
nuclio-build -n nuclio-hello-world --push <registryURL:port> $GOPATH/src/github.com/nuclio/nuclio-sdk/examples/hello-world
```

Run the processor locally and then access port 8080 to test it out:
```
docker run -p 8080:8080 nuclio-hello-world:latest
```

Create a function.yaml file with the following contents:
```
apiVersion: nuclio.io/v1
kind: Function
metadata:
  name: handler
spec:
  replicas: 1
  image: localhost:5000/nuclio-hello-world:latest
  httpPort: 31010
```

Create it with kubectl:
```
kubectl create -f function.yaml
```

You can now do an HTTP GET on `<k8s node ip>:31010` to trigger the function.
