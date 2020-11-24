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
	"fmt"
	"strings"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
)

type python struct {
	*runtime.AbstractRuntime
}

// GetName returns the name of the runtime, including version if applicable
func (p *python) GetName() string {
	return "python"
}

// GetProcessorDockerfileInfo returns information required to build the processor Dockerfile
func (p *python) GetProcessorDockerfileInfo(onbuildImageRegistry string) (*runtime.ProcessorDockerfileInfo, error) {

	processorDockerfileInfo := runtime.ProcessorDockerfileInfo{}
	pythonCommonModules := []string{
		"nuclio-sdk",
		"msgpack",
	}

	if p.FunctionConfig.Spec.Runtime == "python:2.7" {
		p.Logger.Warn("Python 2.7 runtime is deprecated. " +
			"Nuclio will drop support for Python 2.7 runtime as of version 1.6.0. " +
			"Please migrate your code to Python 3.6")
		processorDockerfileInfo.BaseImage = "python:2.7-alpine"
	} else {
		processorDockerfileInfo.BaseImage = "python:3.6"
	}

	processorDockerfileInfo.ImageArtifactPaths = map[string]string{
		"handler": "/opt/nuclio",
	}

	// fill onbuild artifact
	artifact := runtime.Artifact{
		Name: "python-onbuild",
		Image: fmt.Sprintf("%s/nuclio/handler-builder-python-onbuild:%s-%s",
			onbuildImageRegistry,
			p.VersionInfo.Label,
			p.VersionInfo.Arch),
		Paths: map[string]string{
			"/home/nuclio/bin/processor": "/usr/local/bin/processor",
			"/home/nuclio/bin/py":        "/opt/nuclio/",
		},
	}
	processorDockerfileInfo.OnbuildArtifacts = []runtime.Artifact{artifact}

	processorDockerfileInfo.Directives = map[string][]functionconfig.Directive{
		"postCopy": {
			{
				Kind: "RUN",
				Value: fmt.Sprintf(
					"pip install %s --no-index --find-links /opt/nuclio/whl",
					strings.Join(pythonCommonModules, " ")),
			},
		},
	}

	return &processorDockerfileInfo, nil
}
