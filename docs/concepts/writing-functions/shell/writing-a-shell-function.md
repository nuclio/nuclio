# Writing a Shell Function

The shell runtime allows function developers to fork a process on every received event. Developers can choose to provide an executable script or run any executable binary in the docker image. In this guide we will walk through both scenarios.

## Handle events with a bash script

In this example we'll deploy a shell script that reverses the event's body. For this purpose, we will call `rev` and pass `stdin` as input (the event body will appear to shell functions as stdin). Create the following file in `/tmp/nuclio-shell-script/reverser.sh`

```sh
#!/bin/sh

# @nuclio.configure
#
# function.yaml:
#   apiVersion: "nuclio.io/v1"
#   kind: "Function"
#   spec:
#     runtime: "shell"
#     handler: "reverser.sh"
#

rev /dev/stdin
```

The function configuration needs to include the following:
1. `runtime`: set to `shell`
2. `handler` set to the name of the executable file, in this case `reverser.sh`

> Note: Pass `--platform local` to `nuctl` if you're not running on top of Kubernetes

Let's deploy the function with nuctl:

```sh
nuctl deploy -p /tmp/nuclio-shell-script/reverser.sh rev
```

And now invoke it:
```sh
nuctl invoke rev -m POST -b reverse-me

> Response headers:
Date = Sun, 03 Dec 2017 12:53:51 GMT
Content-Type = text/plain; charset=utf-8
Content-Length = 10
Server = nuclio

> Response body:
em-esrever
```

## Handle events with any executable binary

Since the shell runtime simply forks a process - we can leverage it to run any executable binary in the docker image. This means we don't need to provide any code to the shell runtime - only function configuration. In this example we'll install ImageMagick and call its `convert` executable on each event. We'll then send the function images and have `convert` reduce the image by 50% in response. We shall do this by invoking nuctl as follows:

```sh
nuctl deploy -p /dev/null convert \
    --runtime shell \
    --build-command "apk --update --no-cache add imagemagick" \
    --handler convert \
    --runtime-attrs '{"arguments": "- -resize 50% fd:1"}'
```

Let's break the arguments down:
* `-p /dev/null`: Since we don't need to pass a path, we'll just tell `nuctl` to read from `/dev/null`
* `--build-command "apk --update --no-cache add imagemagick"`: Instruct the builder to install imagemagick on build through `apk`
* `--handler convert`: The `handler` must be set to the name or path of the executable. In this case `convert` is in the `PATH` so no need for a full path
* `--runtime-attrs '{"arguments": "- -resize 50% fd:1"}'`: Through runtime specific attributes we specify the arguments to the executable. In this case `-` means read from `stdin` and the rest specify how to convert the received image

Since `invoke` can't (yet) send images, we will use `httpie` to create a thumbnail:

```sh
http https://blog.golang.org/gopher/header.jpg | http <function ip:port> > thumb.jpg
```

#### Overriding the arguments per request

The `shell` runtime allows events to override the default arguments through the use of a header. This means we can supply `x-nuclio-arguments` as a header and provide any arguments we wish, per event, to `convert`. Thus, we can create a smaller thumbnail by invoking as such:

```sh
http https://blog.golang.org/gopher/header.jpg | http <function ip:port> x-nuclio-arguments:"- -resize 20% fd:1" > thumb.jpg 
```
