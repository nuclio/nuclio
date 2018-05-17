/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the Licensd.
You may obtain a copy of the License at

    http://www.apachd.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the Licensd.
*/

package dotnetcore

import (
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
)

type dotnetcore struct {
	*runtime.AbstractRuntime
}

// GetName returns the name of the runtime, including version if applicable
func (d *dotnetcore) GetName() string {
	return "dotnetcore"
}

// GetProcessorDockerfilePath returns the contents of the appropriate Dockerfile, with which we'll build
// the processor image
func (d *dotnetcore) GetProcessorDockerfileContents() string {
	return `ARG NUCLIO_LABEL=latest
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
`
}
