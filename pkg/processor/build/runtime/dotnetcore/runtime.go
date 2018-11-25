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
	"fmt"

	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/version"
)

type dotnetcore struct {
	*runtime.AbstractRuntime
}

// GetName returns the name of the runtime, including version if applicable
func (d *dotnetcore) GetName() string {
	return "dotnetcore"
}

// GetProcessorDockerfileInfo returns information required to build the processor Dockerfile
func (d *dotnetcore) GetProcessorDockerfileInfo(versionInfo *version.Info) (*runtime.ProcessorDockerfileInfo, error) {
	processorDockerfileInfo := runtime.ProcessorDockerfileInfo{}

	// format the onbuild image
	processorDockerfileInfo.OnbuildImage = fmt.Sprintf("quay.io/nuclio/handler-builder-dotnetcore-onbuild:%s-%s",
		versionInfo.Label,
		versionInfo.Arch)

	// set the default base image
	processorDockerfileInfo.BaseImage = "microsoft/dotnet:2-runtime"
	processorDockerfileInfo.OnbuildArtifactPaths = map[string]string{
		"/home/nuclio/bin/processor":             "/usr/local/bin/processor",
		"/home/nuclio/bin/wrapper":               "/opt/nuclio/wrapper",
		"/home/nuclio/bin/handler":               "/opt/nuclio/handler",
		"/home/nuclio/src/nuclio-sdk-dotnetcore": "/opt/nuclio/nuclio-sdk-dotnetcore",
	}

	return &processorDockerfileInfo, nil
}
