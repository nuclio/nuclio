# .NET Core Reference

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
        <TargetFramework>net6.0</TargetFramework>
        <GenerateAssemblyInfo>false</GenerateAssemblyInfo>
        <LangVersion>10.0</LangVersion>
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
For more details about `dotnet add package`, see the [Microsoft documentation](https://docs.microsoft.com/en-us/dotnet/core/tools/dotnet-add-package).

## Dockerfile

See [Deploying Functions from a Dockerfile](../../../tasks/deploy-functions-from-dockerfile.md).

```
ARG NUCLIO_LABEL=0.5.6
ARG NUCLIO_ARCH=amd64
ARG NUCLIO_BASE_IMAGE=mcr.microsoft.com/dotnet/sdk:6.0
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

