# .NET Core

This document describes the specific .NET Core build and deploy configurations.

#### In this document

- [Function and handler](#function-and-handler)
- [Dockerfile](#dockerfile)

## Function and handler

```cs
using System;
using Nuclio.Sdk;

public class nuclio
{
    public object empty(Context context, Event eventBase)
    {
        return new Response()
        {
            StatusCode = 200,
            ContentType = "application/text",
            Body = ""
        };
    }
}
```

The `handler` field is of the form `<class>:<entrypoint>`. In the example above, the handler is `nuclio:empty`.

## Project file

To use or import external dependencies, create a **handler.csproj** file that lists the required dependencies, alongside your function-handler file.

For example, the following file defines a dependency on the `Microsoft.NET.Sdk` package:
```xml
<Project Sdk="Microsoft.NET.Sdk">
    <PropertyGroup>
        <TargetFramework>net7.0</TargetFramework>
        <GenerateAssemblyInfo>false</GenerateAssemblyInfo>
        <LangVersion>11.0</LangVersion>
    </PropertyGroup>
    <ItemGroup>
        <PackageReference Include="Newtonsoft.Json" Version="12.0.2"/>
    </ItemGroup>
</Project>
```

With this example **handler.csproj** file, you can use the `Newtonsoft.Json` package as follows:

```cs
using Newtonsoft.Json;
...
JsonConvert.SerializeObject(...);
```

Adding more dependencies is made easy using `dotnet add package <package name>`.
For more details about `dotnet add package`, see the [Microsoft documentation](https://learn.microsoft.com/en-us/dotnet/core/tools/dotnet-add-package).

## Dockerfile

See [Deploying Functions from a Dockerfile](../../../tasks/deploy-functions-from-dockerfile.md).

```
ARG NUCLIO_LABEL=0.5.6
ARG NUCLIO_ARCH=amd64
ARG NUCLIO_BASE_IMAGE=mcr.microsoft.com/dotnet/sdk:7.0
ARG NUCLIO_ONBUILD_IMAGE=nuclio/handler-builder-dotnetcore-onbuild:${NUCLIO_LABEL}-${NUCLIO_ARCH}

# Supplies processor uhttpc, used for healthcheck
FROM nuclio/uhttpc:0.0.1-amd64 as uhttpc

# Builds source, supplies processor binary and handler plugin
FROM ${NUCLIO_ONBUILD_IMAGE} as builder

# From the base image
FROM ${NUCLIO_BASE_IMAGE}

# Copy required objects from the suppliers
COPY --from=builder /home/nuclio/bin/processor /usr/local/bin/processor
COPY --from=builder /home/nuclio/bin/wrapper /opt/nuclio/wrapper
COPY --from=builder /home/nuclio/bin/handler /opt/nuclio/handler
COPY --from=builder /home/nuclio/src/nuclio-sdk-dotnetcore /opt/nuclio/nuclio-sdk-dotnetcore
COPY --from=uhttpc /home/nuclio/bin/uhttpc /usr/local/bin/uhttpc

# Readiness probe
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://127.0.0.1:8082/ready || exit 1

# Run processor with configuration and platform configuration
CMD [ "processor" ]
```

# Writing a .NET Core 7.0 Function

This guide uses practical examples to guide you through the process of writing serverless .NET Core functions.

#### In this document

- [Overview](#overview)
- [Deploy a .NET Core function](#deploy-a-net-core-function)
- [See also](#see-also)

## Overview

The .NET Core runtime allows function developers to create serverless functions using [.NET Core 3.1](https://dotnet.microsoft.com/). This guide walks you through the function-creation process.

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

Run the following command to deploy the function with the Nuclio CLI (`nuctl`).

> **Note:** If you're not running on top of Kubernetes, pass the `--platform local` option to `nuctl`.

```sh
nuctl deploy -p /tmp/nuclio-dotnetcore-script/reverser.cs reverser
```

You can also remove the settings from `reverser.cs` and run the following command:

```sh
nuctl deploy -p /tmp/nuclio-dotnetcore-script/reverser.cs --runtime dotnetcore --handler nuclio:reverser reverser
```

And now, use the `nuctl` CLI to invoke the function:
```sh
$ nuctl invoke reverser -m POST -b reverse-me

> Response headers:
Date = Sun, 03 Dec 2017 12:53:51 GMT
Content-Type = text/plain; charset=utf-8
Content-Length = 10
Server = nuclio

> Response body:
em-esrever
```

## See also

- [Deploying Functions](../../../tasks/deploying-functions.md)
- [Function-Configuration Reference](../../../reference/function-configuration/function-configuration-reference.md)
