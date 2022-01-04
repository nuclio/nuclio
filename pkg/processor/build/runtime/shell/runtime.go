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

package shell

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/build/runtimeconfig"
)

type shell struct {
	*runtime.AbstractRuntime
}

// GetName returns the name of the runtime, including version if applicable
func (s *shell) GetName() string {
	return "shell"
}

// GetProcessorDockerfileInfo returns information required to build the processor Dockerfile
func (s *shell) GetProcessorDockerfileInfo(runtimeConfig *runtimeconfig.Config, onbuildImageRegistry string) (*runtime.ProcessorDockerfileInfo, error) {

	processorDockerfileInfo := runtime.ProcessorDockerfileInfo{}

	// set the default base image
	processorDockerfileInfo.BaseImage = "alpine:3.15"

	// fill onbuild artifact
	artifact := runtime.Artifact{
		Name: "nuclio-processor",
		Image: fmt.Sprintf("%s/nuclio/processor:%s-%s",
			onbuildImageRegistry,
			s.VersionInfo.Label,
			s.VersionInfo.Arch),
		Paths: map[string]string{
			"/home/nuclio/bin/processor": "/usr/local/bin/processor",
		},
	}
	processorDockerfileInfo.OnbuildArtifacts = []runtime.Artifact{artifact}

	processorDockerfileInfo.ImageArtifactPaths = map[string]string{
		"handler": "/opt/nuclio",
	}

	return &processorDockerfileInfo, nil
}

// GetProcessorImageObjectPaths returns the paths of all objects that should reside in the handler
// directory
func (s *shell) GetHandlerDirObjectPaths() []string {
	if s.FunctionConfig.Spec.Build.Path != "/dev/null" {
		return s.AbstractRuntime.GetHandlerDirObjectPaths()
	}

	return []string{}
}
