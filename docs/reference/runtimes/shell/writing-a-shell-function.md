# Writing a Shell Function

This guide uses practical examples to guide you through the process of writing serverless shell functions.

#### In this document

- [Overview](#overview)
- [Handle events with a bash script](#handle-events-with-a-bash-script)
- [Handle events with any executable binary](#handle-events-with-any-executable-binary)
- [See also](#see-also)

## Overview

The shell runtime allows function developers to fork a process on every received event. Developers can choose to provide an executable script or run any executable binary in the Docker image. This guide walks you through both scenarios.

## Handle events with a bash script

This example guides you through the steps for deploying a shell script that reverses the event's body. To implement this, you call `rev` and pass `stdin` as input; (the event body will appear to shell functions as `stdin`).

Create a **/tmp/nuclio-shell-script/reverser.sh** file with the following code:

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

1. `runtime` - set to `shell`.
2. `handler` - set to the name of the executable file. In this example, the file is **reverser.sh**.

Run the following command to deploy the function with the [`nuctl`](/docs/reference/nuctl/nuctl.md) nuclio CLI:
> Note: if you're not running on top of Kubernetes, pass the `--platform local` option to `nuctl`.

```sh
nuctl deploy -p /tmp/nuclio-shell-script/reverser.sh rev
```

And now, use the `nuctl` CLI to invoke the function:
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

Because the shell runtime simply forks a process, you can leverage it to run any executable binary in the Docker image. This means that you don't need to provide any code to the shell runtime, only a function configuration. In this example, you install the [ImageMagick](https://www.imagemagick.org/script/index.php) utility and call its `convert` executable on each event. You then send the function images and use `convert` to reduce the image by 50% in the response. You do this by invoking the `nuctl` CLI as follows:

```sh
nuctl deploy -p /dev/null convert \
    --runtime shell \
    --build-command "apk --update --no-cache add imagemagick" \
    --handler convert \
    --runtime-attrs '{"arguments": "- -resize 50% fd:1"}'
```

Following is an explanation of the options used in the command:

- `-p /dev/null` - because you don't need to pass a path, you just instruct `nuctl` to read from `/dev/null`.
- `--build-command "apk --update --no-cache add imagemagick"` - instruct the builder to install ImageMagick on the build through `apk`.
- `--handler convert` - the `handler` must be set to the name or path of the executable. In this example, `convert` is in the environment `PATH` so there's no need for a full path.
- `--runtime-attrs '{"arguments": "- -resize 50% fd:1"}'` - through runtime specific attributes, you specify the arguments for the executable. In this example, `-` instructs the runtime to read from `stdin`, and the rest of the arguments specify how to convert the received image.

Because `invoke` can't (yet) send images, use [HTTPie](https://httpie.org/) to create a thumbnail file; replace the `<function ip:port>` placeholder with your function-URL information:

```sh
http https://blog.golang.org/gopher/header.jpg | http <function ip:port> > thumb.jpg
```

### Overriding the arguments per request

The `shell` runtime allows events to override the default arguments through the use of a header. This means that you can supply `x-nuclio-arguments` as a header and provide any `convert` arguments that you wish, per event. Thus, you can create, for example, a smaller thumbnail file by using the following invocation command; replace the `<function ip:port>` placeholder with your function-URL information:

```sh
http https://blog.golang.org/gopher/header.jpg | http <function ip:port> x-nuclio-arguments:"- -resize 20% fd:1" > thumb.jpg 
```

## See also

- [Deploying Functions](/docs/tasks/deploying-functions.md)
- [Function-Configuration Reference](/docs/reference/function-configuration/function-configuration-reference.md)

