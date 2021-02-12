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

package deno

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
)

type deno struct {
	*runtime.AbstractRuntime
}

// GetName returns the name of the runtime, including version if applicable
func (n *deno) GetName() string {
	return "deno"
}

// GetProcessorDockerfileInfo returns information required to build the processor Dockerfile
func (n *deno) GetProcessorDockerfileInfo(onbuildImageRegistry string) (*runtime.ProcessorDockerfileInfo, error) {

	processorDockerfileInfo := runtime.ProcessorDockerfileInfo{}

	// set the default base image
	processorDockerfileInfo.BaseImage = "hayd/alpine-deno:1.5.2"

	processorDockerfileInfo.ImageArtifactPaths = map[string]string{
		"handler": "/opt/nuclio",
	}

	// fill onbuild artifact
	artifact := runtime.Artifact{
		Name: "deno-onbuild",
		Image: fmt.Sprintf("%s/nuclio/handler-builder-deno-onbuild:%s-%s",
			onbuildImageRegistry,
			n.VersionInfo.Label,
			n.VersionInfo.Arch),
		Paths: map[string]string{
			"/home/nuclio/bin/processor":  "/usr/local/bin/processor",
			"/home/nuclio/bin/wrapper.ts": "/opt/nuclio/wrapper.ts",
		},
	}
	processorDockerfileInfo.OnbuildArtifacts = []runtime.Artifact{artifact}

	processorDockerfileInfo.Directives = map[string][]functionconfig.Directive{
		//"postCopy": {
		//{Kind: "ENV", Value: "NODE_PATH=/usr/local/lib/node_modules"},
		//},
	}

	return &processorDockerfileInfo, nil
}
