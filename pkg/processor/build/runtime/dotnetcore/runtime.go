/*
Copyright 2023 The Nuclio Authors.

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
	"fmt"

	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/build/runtimeconfig"
)

type dotnetcore struct {
	*runtime.AbstractRuntime
}

// GetName returns the name of the runtime, including version if applicable
func (d *dotnetcore) GetName() string {
	return "dotnetcore"
}

// GetProcessorDockerfileInfo returns information required to build the processor Dockerfile
func (d *dotnetcore) GetProcessorDockerfileInfo(runtimeConfig *runtimeconfig.Config, onbuildImageRegistry string) (*runtime.ProcessorDockerfileInfo, error) {

	processorDockerfileInfo := runtime.ProcessorDockerfileInfo{}

	// fill onbuild artifact
	artifact := runtime.Artifact{
		Name: "dotnetcore-onbuild",
		Image: fmt.Sprintf("%s/nuclio/handler-builder-dotnetcore-onbuild:%s-%s",
			onbuildImageRegistry,
			d.AbstractRuntime.VersionInfo.Label,
			d.AbstractRuntime.VersionInfo.Arch),
		Paths: map[string]string{
			"/home/nuclio/bin/processor":             "/usr/local/bin/processor",
			"/home/nuclio/bin/wrapper":               "/opt/nuclio/wrapper",
			"/home/nuclio/bin/handler":               "/opt/nuclio/handler",
			"/home/nuclio/src/nuclio-sdk-dotnetcore": "/opt/nuclio/nuclio-sdk-dotnetcore",
		},
	}
	processorDockerfileInfo.OnbuildArtifacts = []runtime.Artifact{artifact}

	// set the default base image
	processorDockerfileInfo.BaseImage = "mcr.microsoft.com/dotnet/runtime:7.0"
	return &processorDockerfileInfo, nil
}
