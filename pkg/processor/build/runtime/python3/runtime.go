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

package python3

import (
	"fmt"
	"path"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/version"
)

type python3 struct {
	*runtime.AbstractRuntime
}

// GetProcessorBaseImageName returns the image name of the default processor base image
func (p *python3) GetProcessorBaseImageName() (string, error) {

	// get the version we're running so we can pull the compatible image
	versionInfo, err := version.Get()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get version")
	}

	baseImageName := fmt.Sprintf("nuclio/handler-python3-stretch:%s-%s", versionInfo.Label, versionInfo.Arch)

	// make sure the image exists. don't pull if instructed not to
	if !p.FunctionConfig.Spec.Build.NoBaseImagesPull {
		if err := p.DockerClient.PullImage(baseImageName); err != nil {
			return "", errors.Wrapf(err, "Can't pull %q", baseImageName)
		}
	}

	return baseImageName, nil
}

// DetectFunctionHandlers returns a list of all the handlers
// in that directory given a path holding a function (or functions)
func (p *python3) DetectFunctionHandlers(functionPath string) ([]string, error) {
	return []string{p.getFunctionHandler()}, nil
}

// GetProcessorImageObjectPaths returns a map of objects the runtime needs to copy into the processor image
// the key can be a dir, a file or a url of a file
// the value is an absolute path into the docker image
func (p *python3) GetProcessorImageObjectPaths() map[string]string {
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
func (p *python3) GetExtension() string {
	return "py"
}

// GetName returns the name of the runtime, including version if applicable
func (p *python3) GetName() string {
	return "python3"
}

func (p *python3) getFunctionHandler() string {
	// use the function path: /some/path/func.py -> func
	functionFileName := path.Base(p.FunctionConfig.Spec.Build.Path)
	functionFileName = functionFileName[:len(functionFileName)-len(path.Ext(functionFileName))]

	// take that file name without extension and add a default "handler"
	// TODO: parse the python sources for this
	return fmt.Sprintf("%s:%s", functionFileName, "handler")
}
