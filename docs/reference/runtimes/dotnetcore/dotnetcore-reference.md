# .NET Core reference

This document describes .NET Core-specific build and deploy configurations.

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

The `handler` field takes the form of `class:entrypoint`. In the above, the handler is `nuclio:empty`. 

## Dockerfile
See [deploying Functions from Dockerfile](/docs/tasks/deploy-functions-from-dockerfile.md).

```
ARG NUCLIO_LABEL=0.5.0
ARG NUCLIO_ARCH=amd64
ARG NUCLIO_BASE_IMAGE=microsoft/dotnet:2-runtime
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
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://localhost:8082/ready || exit 1

# Run processor with configuration and platform configuration
CMD [ "processor", "--config", "/etc/nuclio/config/processor/processor.yaml", "--platform-config", "/etc/nuclio/config/platform/platform.yaml" ]
```
