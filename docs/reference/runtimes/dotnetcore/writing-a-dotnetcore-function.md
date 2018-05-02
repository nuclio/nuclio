# Writing a .NET Core 2 Function

This guide uses practical examples to guide you through the process of writing serverless .NET Core functions.

#### In this document

- [Overview](#overview)
- [Deploy a .NET Core function](#deploy-a-net-core-function)
- [See also](#see-also)

## Overview

The .NET Core runtime allows function developers to create serverless functions using [.NET Core 2](https://dotnet.github.io/). This guide walks you through the function-creation process.

## Deploy a .NET Core function

This example guides you through the steps for deploying a .NET Core code that reverses the event's body. To implement this, you call `reverser` and pass an input;

Create a **/tmp/nuclio-dotnetcore-script/reverser.cs** file with the following code:

```csharp
// @nuclio.configure
//
// function.yaml:
//   spec:
//     runtime: dotnetcore
//     handler: nuclio:reverser

using System;
using Nuclio.Sdk;

public class nuclio
{
    public string reverser(Context context, Event eventBase)
    {
        var charArray = eventBase.GetBody().ToCharArray();
        Array.Reverse(charArray);
        return new string(charArray);
    }
}
```

The function configuration needs to include the following:

1. `runtime` - set to `dotnetcore`.
2. `handler` - set to the name of the class and the name of the method . In this example, the handler is **nuclio:reverser**.

Run the following command to deploy the function with the [`nuctl`](/docs/reference/nuctl/nuctl.md) nuclio CLI:

> Note: If you're not running on top of Kubernetes, pass the `--platform local` option to `nuctl`.

```sh
nuctl deploy -p /tmp/nuclio-dotnetcore-script/reverser.cs reverser
```

You can also remove the settings from `reverser.cs` and run the following command:

```sh
nuctl deploy -p /tmp/nuclio-dotnetcore-script/reverser.cs --runtime dotnetcore --handler nuclio:reverser reverser
```

And now, use the `nuctl` CLI to invoke the function:
```sh
nuctl invoke reverser -m POST -b reverse-me

> Response headers:
Date = Sun, 03 Dec 2017 12:53:51 GMT
Content-Type = text/plain; charset=utf-8
Content-Length = 10
Server = nuclio

> Response body:
em-esrever
```

## See also

- [Deploying Functions](/docs/tasks/deploying-functions.md)
- [Function-Configuration Reference](/docs/reference/function-configuration/function-configuration-reference.md)

