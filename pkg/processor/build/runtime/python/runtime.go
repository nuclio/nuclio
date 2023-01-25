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
	"io/ioutil"
	"path"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/build/runtimeconfig"

	"github.com/nuclio/errors"
)

const pipCAFileLocation = "/etc/ssl/certs/nuclio/pip-ca-certificates.crt"

type python struct {
	*runtime.AbstractRuntime
}

// GetName returns the name of the runtime, including version if applicable
func (p *python) GetName() string {
	return "python"
}

// GetProcessorDockerfileInfo returns information required to build the processor Dockerfile
func (p *python) GetProcessorDockerfileInfo(runtimeConfig *runtimeconfig.Config,
	onbuildImageRegistry string) (*runtime.ProcessorDockerfileInfo, error) {
	var installSDKDependenciesCommand string
	var baseImage string

	srcOnbuildWheelsPath := "/home/nuclio/bin/py-whl"
	destOnbuildWheelsPath := "/opt/nuclio/whl"
	pythonCommonModules := []string{
		"nuclio-sdk",
		"msgpack",
	}

	pipInstallArgs := []string{
		"--no-index",
		"--find-links", destOnbuildWheelsPath,
	}

	_, runtimeVersion := common.GetRuntimeNameAndVersion(p.FunctionConfig.Spec.Runtime)

	switch runtimeVersion {
	case "3.6":
		baseImage = "python:3.6"

		p.Logger.Warn("Python 3.6 runtime is deprecated and will soon not be supported. " +
			"Migrate your code and use Python 3.9 runtime (`python:3.9`) or higher")
		installSDKDependenciesCommand = fmt.Sprintf("pip install %s %s",
			strings.Join(pythonCommonModules, " "),
			strings.Join(pipInstallArgs, " "),
		)

	default:
		baseImage = fmt.Sprintf("python:%s", runtimeVersion)

		// use specific wheel files path
		srcOnbuildWheelsPath = fmt.Sprintf("/home/nuclio/bin/py%s-whl", runtimeVersion)

		// dont require special privileges
		// TODO: enable when provide USER directive pre copying artifacts
		// since the build user is root, while running container user might be different
		// and hence the packages wont be available to the running user.
		// to overcome it, suggest to add `PYTHONUSERBASE=/some/path` with the running container user access
		// pipInstallArgs = append(pipInstallArgs, "--user")

		// ensure pip is installed on python interpreter
		installPipCommand := fmt.Sprintf("python %[1]s/$(basename %[1]s/pip-*.whl)/pip install pip %[2]s",
			destOnbuildWheelsPath,
			strings.Join(pipInstallArgs, " "))

		// run pip from the python interpreter
		installSDKDependenciesCommand = fmt.Sprintf("%s && python -m pip install %s %s",
			installPipCommand,
			strings.Join(pythonCommonModules, " "),
			strings.Join(pipInstallArgs, " "))

	}

	// fill onbuild artifact
	processorDockerfileInfo := runtime.ProcessorDockerfileInfo{
		BaseImage: baseImage,
		ImageArtifactPaths: map[string]string{
			"handler": "/opt/nuclio",
		},
		OnbuildArtifacts: []runtime.Artifact{
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
		},
		Directives: map[string][]functionconfig.Directive{
			"postCopy": {
				{
					Kind:  "RUN",
					Value: installSDKDependenciesCommand,
				},
			},
		},
	}

	// copy pip ca artifact to function image
	if runtimeConfig != nil && runtimeConfig.Python != nil {
		if runtimeConfig.Python.PipCAPath != "" {
			processorDockerfileInfo.Directives["preCopy"] = append(processorDockerfileInfo.Directives["preCopy"],
				functionconfig.Directive{
					Kind:  "COPY",
					Value: fmt.Sprintf("%s %s", path.Base(pipCAFileLocation), pipCAFileLocation),
				})
		}
	}

	return &processorDockerfileInfo, nil
}

// GetRuntimeBuildArgs returns python specific build args directives
func (p *python) GetRuntimeBuildArgs(runtimeConfig *runtimeconfig.Config) map[string]string {
	buildArgs := p.AbstractRuntime.GetRuntimeBuildArgs(runtimeConfig)

	if runtimeConfig != nil && runtimeConfig.Python != nil {

		// enrich build arg with pip ca file path
		if runtimeConfig.Python.PipCAPath != "" {
			buildArgs["PIP_CERT"] = pipCAFileLocation
		}

		// enrich build args with runtime specific build args
		for key, value := range runtimeConfig.Python.BuildArgs {
			buildArgs[key] = value
		}
	}
	return buildArgs
}

func (p *python) OnAfterStagingDirCreated(runtimeConfig *runtimeconfig.Config, stagingDir string) error {
	if runtimeConfig != nil && runtimeConfig.Python != nil {
		PipCAContents, err := runtimeConfig.Python.GetPipCAContents()
		if err != nil {
			return errors.Wrap(err, "Failed to get pip ca contents")
		}

		destPath := path.Join(stagingDir, path.Base(pipCAFileLocation))
		p.Logger.DebugWith("Writing pip ca contents", "destPath", destPath)
		if err := ioutil.WriteFile(destPath, PipCAContents, 0644); err != nil {
			return errors.Wrap(err, "Failed to write pip ca contents to file")
		}
	}
	return p.AbstractRuntime.OnAfterStagingDirCreated(runtimeConfig, stagingDir)
}
