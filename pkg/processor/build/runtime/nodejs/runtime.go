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

package nodejs

import (
	"fmt"
	"path"

	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
)

type nodejs struct {
	*runtime.AbstractRuntime
}

// DetectFunctionHandlers returns a list of all the handlers
// in that directory given a path holding a function (or functions)
func (p *nodejs) DetectFunctionHandlers(functionPath string) ([]string, error) {
	return []string{p.getFunctionHandler()}, nil
}

// GetName returns the name of the runtime, including version if applicable
func (p *nodejs) GetName() string {
	return "nodejs"
}

// GetProcessorDockerfilePath returns the contents of the appropriate Dockerfile, with which we'll build
// the processor image
func (p *nodejs) GetProcessorDockerfileContents() string {
	return `
ARG NUCLIO_TAG=latest
ARG NUCLIO_ARCH=amd64
ARG NUCLIO_BASE_IMAGE=node:9.3.0-alpine

# Supplies processor uhttpc, used for healthcheck
FROM nuclio/uhttpc:latest-amd64 as uhttpc

# Supplies processor binary, wrapper
FROM nuclio/handler-builder-nodejs-onbuild:${NUCLIO_TAG}-${NUCLIO_ARCH} as processor

# From the base image
FROM ${NUCLIO_BASE_IMAGE}

# Copy required objects from the suppliers
COPY --from=processor /home/nuclio/bin/processor /usr/local/bin/processor
COPY --from=processor /home/nuclio/bin/wrapper.js /opt/nuclio/wrapper.js
COPY --from=uhttpc /home/nuclio/bin/uhttpc /usr/local/bin/uhttpc

# Copy the handler directory to /opt/nuclio
COPY handler /opt/nuclio

# Readiness probe
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://localhost:8082/ready || exit 1

# Set node modules path
ENV NODE_PATH=/usr/local/lib/node_modules

# Run processor with configuration and platform configuration
CMD [ "processor", "--config", "/etc/nuclio/config/processor/processor.yaml", "--platform-config", "/etc/nuclio/config/platform/platform.yaml" ]
`
}

func (p *nodejs) getFunctionHandler() string {

	// use the function path: /some/path/func.py -> func
	functionFileName := path.Base(p.FunctionConfig.Spec.Build.Path)
	functionFileName = functionFileName[:len(functionFileName)-len(path.Ext(functionFileName))]

	// take that file name without extension and add a default "handler"
	// TODO: parse the nodejs sources for this
	return fmt.Sprintf("%s:%s", functionFileName, "handler")
}
