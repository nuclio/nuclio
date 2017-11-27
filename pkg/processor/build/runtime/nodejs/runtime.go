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

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/version"
)

const (
	defaultRuntimeVersion = "9.2"
	defaultBaseImageName  = "stretch"
)

var (
	supportedRuntimes = map[string]bool{
		defaultRuntimeVersion: true,
	}

	supportedImages = map[string]bool{
		defaultBaseImageName: true,
	}
)

type nodejs struct {
	*runtime.AbstractRuntime
}

// GetProcessorBaseImageName returns the image name of the default processor base image
func (node *nodejs) GetProcessorBaseImageName() (string, error) {

	// get the version we're running so we can pull the compatible image
	versionInfo, err := version.Get()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get version")
	}

	_, runtimeVersion := node.GetRuntimeNameAndVersion()

	// try to get base image name
	baseImageName, err := getBaseImageName(versionInfo,
		runtimeVersion,
		node.FunctionConfig.Spec.Build.BaseImageName)

	if err != nil {
		return "", errors.Wrap(err, "Failed to get base image name")
	}

	// make sure the image exists. don't pull if instructed not to
	if !node.FunctionConfig.Spec.Build.NoBaseImagesPull {
		if err := node.DockerClient.PullImage(baseImageName); err != nil {
			return "", errors.Wrapf(err, "Can't pull %q", baseImageName)
		}
	}

	return baseImageName, nil
}

// DetectFunctionHandlers returns a list of all the handlers
// in that directory given a path holding a function (or functions)
func (node *nodejs) DetectFunctionHandlers(functionPath string) ([]string, error) {
	return []string{node.getFunctionHandler()}, nil
}

// GetProcessorImageObjectPaths returns a map of objects the runtime needs to copy into the processor image
// the key can be a dir, a file or a url of a file
// the value is an absolute path into the docker image
func (node *nodejs) GetProcessorImageObjectPaths() map[string]string {
	functionPath := node.FunctionConfig.Spec.Build.Path
	dockerPath := path.Join("opt", "nuclio", "handler")

	if common.IsFile(functionPath) {
		dockerPath = path.Join(dockerPath, "handler.js")
	}

	return map[string]string{
		functionPath: dockerPath,
	}
}

// GetExtension returns the source extension of the runtime (e.g. .go)
func (node *nodejs) GetExtension() string {
	return "js"
}

// GetName returns the name of the runtime, including version if applicable
func (node *nodejs) GetName() string {
	return "nodejs"
}

func (node *nodejs) getFunctionHandler() string {
	// TODO: Detect from source code
	return "handler"
}

func getBaseImageName(versionInfo *version.Info,
	runtimeVersion string,
	baseImageName string) (string, error) {

	// if the runtime version contains any value, use it. otherwise default to 3.6
	if runtimeVersion == "" {
		runtimeVersion = defaultRuntimeVersion
	}

	// if base image name not passed, use our
	if baseImageName == "" {
		baseImageName = defaultBaseImageName
	}

	// check runtime
	if ok := supportedRuntimes[runtimeVersion]; !ok {
		return "", fmt.Errorf("Runtime version not supported: %s", runtimeVersion)
	}

	// check base image
	if ok := supportedImages[baseImageName]; !ok {
		return "", fmt.Errorf("Base image not supported: %s", baseImageName)
	}

	return fmt.Sprintf("nuclio/handler-nodejs%s-%s:%s-%s",
		runtimeVersion,
		baseImageName,
		versionInfo.Label,
		versionInfo.Arch), nil
}
