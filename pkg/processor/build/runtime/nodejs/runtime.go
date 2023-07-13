/*
Copyright 2023 The Nuclio Authors.

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

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/build/runtimeconfig"
)

type nodejs struct {
	*runtime.AbstractRuntime
}

// GetName returns the name of the runtime, including version if applicable
func (n *nodejs) GetName() string {
	return "nodejs"
}

// GetProcessorDockerfileInfo returns information required to build the processor Dockerfile
func (n *nodejs) GetProcessorDockerfileInfo(runtimeConfig *runtimeconfig.Config, onbuildImageRegistry string) (*runtime.ProcessorDockerfileInfo, error) {

	processorDockerfileInfo := runtime.ProcessorDockerfileInfo{}

	// set the default base image
	processorDockerfileInfo.BaseImage = "node:10.20-alpine"

	processorDockerfileInfo.ImageArtifactPaths = map[string]string{
		"handler": "/opt/nuclio",
	}

	// fill onbuild artifact
	artifact := runtime.Artifact{
		Name: "nodejs-onbuild",
		Image: fmt.Sprintf("%s/nuclio/handler-builder-nodejs-onbuild:%s-%s",
			onbuildImageRegistry,
			n.VersionInfo.Label,
			n.VersionInfo.Arch),
		Paths: map[string]string{
			"/home/nuclio/bin/processor":  "/usr/local/bin/processor",
			"/home/nuclio/bin/wrapper.js": "/opt/nuclio/wrapper.js",
		},
	}
	processorDockerfileInfo.OnbuildArtifacts = []runtime.Artifact{artifact}

	processorDockerfileInfo.Directives = map[string][]functionconfig.Directive{
		"postCopy": {
			{Kind: "ENV", Value: "NODE_PATH=/usr/local/lib/node_modules"},
		},
	}

	return &processorDockerfileInfo, nil
}
