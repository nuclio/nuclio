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
	"path"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/version"
)

type shell struct {
	*runtime.AbstractRuntime
}

// GetProcessorBaseImageName returns the image name of the default processor base image
func (s *shell) GetProcessorBaseImageName() (string, error) {
	versionInfo, err := version.Get()

	if err != nil {
		return "", errors.Wrap(err, "Failed to get version info")
	}

	return fmt.Sprintf("nuclio/processor-shell-alpine:%s-%s",
		versionInfo.Label,
		versionInfo.Arch), nil
}

// DetectFunctionHandlers returns a list of all the handlers
// in that directory given a path holding a function (or functions)
func (s *shell) DetectFunctionHandlers(functionPath string) ([]string, error) {
	return []string{s.getFunctionHandler()}, nil
}

// GetProcessorImageObjectPaths returns a map of objects the runtime needs to copy into the processor image
// the key can be a dir, a file or a url of a file
// the value is an absolute path into the docker image
func (s *shell) GetProcessorImageObjectPaths() map[string]string {
	functionPath := s.FunctionConfig.Spec.Build.Path

	if common.IsFile(functionPath) {
		return map[string]string{
			functionPath: path.Join("opt", "nuclio", path.Base(functionPath)),
		}
	}

	return map[string]string{
		functionPath: path.Join("opt", "nuclio"),
	}
}

// GetName returns the name of the runtime, including version if applicable
func (s *shell) GetName() string {
	return "shell"
}

func (s *shell) getFunctionHandler() string {
	functionFileName := path.Base(s.FunctionConfig.Spec.Build.Path)

	// take that file name without extension and add a default "handler"
	return fmt.Sprintf("%s:%s", functionFileName, "main")
}
