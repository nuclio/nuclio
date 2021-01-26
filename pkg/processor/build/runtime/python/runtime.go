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
	var installSDKDependenciesCommand string

	srcOnbuildWheelsPath := "/home/nuclio/bin/py-whl"
	destOnbuildWheelsPath := "/opt/nuclio/whl"
	processorDockerfileInfo := runtime.ProcessorDockerfileInfo{
		BaseImage: p.FunctionConfig.Spec.Runtime,
	}
	pythonCommonModules := []string{
		"nuclio-sdk",
		"msgpack",
	}

	processorDockerfileInfo.ImageArtifactPaths = map[string]string{
		"handler": "/opt/nuclio",
	}

	pipInstallArgs := []string{
		"--no-index",
		"--find-links", destOnbuildWheelsPath,
	}

	_, runtimeVersion := p.GetRuntimeNameAndVersion()

	switch runtimeVersion {
	case "3.8":

		// use specific wheel files path
		srcOnbuildWheelsPath = fmt.Sprintf("/home/nuclio/bin/py%s-whl", runtimeVersion)

		// dont require special privileges
		pipInstallArgs = append(pipInstallArgs, "--user")

		// ensure pip is installed on python interpreter
		installPipCommand := fmt.Sprintf("python %[1]s/$(basename %[1]s/pip-*.whl)/pip install pip %[2]s",
			destOnbuildWheelsPath,
			strings.Join(pipInstallArgs, " "))

		// run pip from the python interpreter
		installSDKDependenciesCommand = fmt.Sprintf("%s && python -m pip install %s %s",
			installPipCommand,
			strings.Join(pythonCommonModules, " "),
			strings.Join(pipInstallArgs, " "))

	default:

		// true for python & python:3.6
		installSDKDependenciesCommand = fmt.Sprintf("pip install %s %s",
			strings.Join(pythonCommonModules, " "),
			strings.Join(pipInstallArgs, " "),
		)
	}

	// fill onbuild artifact
	processorDockerfileInfo.OnbuildArtifacts = []runtime.Artifact{
		{
			Name: "python-onbuild",
			Image: fmt.Sprintf("%s/nuclio/handler-builder-python-onbuild:%s-%s",
				onbuildImageRegistry,
				p.VersionInfo.Label,
				p.VersionInfo.Arch),
			Paths: map[string]string{
				"/home/nuclio/bin/processor": "/usr/local/bin/processor",
				"/home/nuclio/bin/py":        "/opt/nuclio/",
				srcOnbuildWheelsPath:         destOnbuildWheelsPath,
			},
		},
	}

	processorDockerfileInfo.Directives = map[string][]functionconfig.Directive{
		"postCopy": {
			{
				Kind:  "RUN",
				Value: installSDKDependenciesCommand,
			},
		},
	}

	return &processorDockerfileInfo, nil
}
