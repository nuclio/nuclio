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

package pypy

import (
	"fmt"
	"path"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/version"
)

const (
	defaultRuntimeVersion = "2-5.9"
	defaultBaseImage      = "jessie"
)

var (
	supportedRuntimes = map[string]bool{
		defaultRuntimeVersion: true,
	}

	supportedImages = map[string]bool{
		defaultBaseImage: true,
	}
)

type pypy struct {
	*runtime.AbstractRuntime
}

// GetProcessorBaseImage returns the image name of the default processor base image
func (p *pypy) GetProcessorBaseImage() (string, error) {

	// get the version we're running so we can pull the compatible image
	versionInfo, err := version.Get()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get version")
	}

	_, runtimeVersion := p.GetRuntimeNameAndVersion()

	// try to get base image name
	baseImage, err := getBaseImage(versionInfo,
		runtimeVersion,
		p.FunctionConfig.Spec.Build.BaseImage)

	if err != nil {
		return "", errors.Wrap(err, "Failed to get base image name")
	}

	// make sure the image exists. don't pull if instructed not to
	if !p.FunctionConfig.Spec.Build.NoBaseImagesPull {
		if err := p.DockerClient.PullImage(baseImage); err != nil {
			return "", errors.Wrapf(err, "Can't pull %q", baseImage)
		}
	}

	return baseImage, nil
}

// DetectFunctionHandlers returns a list of all the handlers
// in that directory given a path holding a function (or functions)
func (p *pypy) DetectFunctionHandlers(functionPath string) ([]string, error) {
	return []string{p.getFunctionHandler()}, nil
}

// GetProcessorImageObjectPaths returns a map of objects the runtime needs to copy into the processor image
// the key can be a dir, a file or a url of a file
// the value is an absolute path into the docker image
func (p *pypy) GetProcessorImageObjectPaths() map[string]string {
	functionPath := p.FunctionConfig.Spec.Build.Path

	if common.IsFile(functionPath) {
		return map[string]string{
			functionPath: path.Join("opt", "nuclio", "handler", path.Base(functionPath)),
		}
	}

	return map[string]string{
		functionPath: path.Join("opt", "nuclio", "handler"),
	}
}

// GetExtension returns the source extension of the runtime (e.g. .go)
func (p *pypy) GetExtension() string {
	return "py"
}

// GetName returns the name of the runtime, including version if applicable
func (p *pypy) GetName() string {
	return "pypy"
}

func (p *pypy) getFunctionHandler() string {

	// use the function path: /some/path/func.py -> func
	functionFileName := path.Base(p.FunctionConfig.Spec.Build.Path)
	functionFileName = functionFileName[:len(functionFileName)-len(path.Ext(functionFileName))]

	// take that file name without extension and add a default "handler"
	// TODO: parse the python sources for this
	return fmt.Sprintf("%s:%s", functionFileName, "handler")
}

func getBaseImage(versionInfo *version.Info,
	runtimeVersion string,
	baseImage string) (string, error) {

	// if the runtime version contains any value, use it. otherwise default to 3.6
	if runtimeVersion == "" {
		runtimeVersion = defaultRuntimeVersion
	}

	// if base image name not passed, use our
	if baseImage == "" {
		baseImage = defaultBaseImage
	}

	// check runtime
	if ok := supportedRuntimes[runtimeVersion]; !ok {
		return "", fmt.Errorf("Runtime version not supported: %s", runtimeVersion)
	}

	// check base image
	if ok := supportedImages[baseImage]; !ok {
		return "", fmt.Errorf("Base image not supported: %s", baseImage)
	}

	return fmt.Sprintf("nuclio/handler-pypy%s-%s:%s-%s",
		runtimeVersion,
		baseImage,
		versionInfo.Label,
		versionInfo.Arch), nil
}
