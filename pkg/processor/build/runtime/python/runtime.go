/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package python

import (
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
)

type python struct {
	*runtime.AbstractRuntime
}

// GetName returns the name of the runtime, including version if applicable
func (p *python) GetName() string {
	return "python"
}

// GetBuildArgs return arguments passed to image builder
func (p *python) GetBuildArgs() (map[string]string, error) {

	// call inherited
	buildArgs, err := p.AbstractRuntime.GetBuildArgs()
	if err != nil {
		return nil, err
	}

	var baseImage string

	switch p.FunctionConfig.Spec.Build.BaseImage {

	// for backwards compatibility
	case "", "alpine":
		if p.FunctionConfig.Spec.Runtime == "python:2.7" {
			baseImage = "python:2.7-alpine"
		} else {
			baseImage = "python:3.6-alpine"
		}

	// for backwards compatibility
	case "jessie":
		if p.FunctionConfig.Spec.Runtime == "python:2.7" {
			baseImage = "python:2.7-jessie"
		} else {
			baseImage = "python:3.6-jessie"
		}

	// if user specified something - use that
	default:
		baseImage = p.FunctionConfig.Spec.Build.BaseImage
	}

	buildArgs["NUCLIO_BASE_IMAGE"] = baseImage

	return buildArgs, nil
}

// GetProcessorDockerfilePath returns the contents of the appropriate Dockerfile, with which we'll build
// the processor image
func (p *python) GetProcessorDockerfileContents() string {
	return `ARG NUCLIO_LABEL=latest
ARG NUCLIO_ARCH=amd64
ARG NUCLIO_BASE_IMAGE=python:3.6-alpine
ARG NUCLIO_ONBUILD_IMAGE=nuclio/handler-builder-python-onbuild:${NUCLIO_LABEL}-${NUCLIO_ARCH}

# Supplies processor uhttpc, used for healthcheck
FROM nuclio/uhttpc:0.0.1-amd64 as uhttpc

# Supplies processor binary, wrapper
FROM ${NUCLIO_ONBUILD_IMAGE} as processor

# From the base image
FROM ${NUCLIO_BASE_IMAGE}

# Copy required objects from the suppliers
COPY --from=processor /home/nuclio/bin/processor /usr/local/bin/processor
COPY --from=processor /home/nuclio/bin/py /opt/nuclio/
COPY --from=uhttpc /home/nuclio/bin/uhttpc /usr/local/bin/uhttpc

# Copy the handler directory to /opt/nuclio
COPY handler /opt/nuclio

# Readiness probe
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://localhost:8082/ready || exit 1

# Run processor with configuration and platform configuration
CMD [ "processor", "--config", "/etc/nuclio/config/processor/processor.yaml", "--platform-config", "/etc/nuclio/config/platform/platform.yaml" ]
`
}
