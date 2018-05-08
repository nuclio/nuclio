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
	"path"

	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
)

type python struct {
	*runtime.AbstractRuntime
}

// DetectFunctionHandlers returns a list of all the handlers
// in that directory given a path holding a function (or functions)
func (p *python) DetectFunctionHandlers(functionPath string) ([]string, error) {
	return []string{p.getFunctionHandler()}, nil
}

// GetName returns the name of the runtime, including version if applicable
func (p *python) GetName() string {
	return "python"
}

// GetBuildArgs return arguments passed to image builder
func (p *python) GetBuildArgs() (map[string]string, error) {
	buildArgs := map[string]string{}

	// call inherited
	buildArgs, err := p.AbstractRuntime.GetBuildArgs()
	if err != nil {
		return nil, err
	}

	baseImage := ""

	switch p.FunctionConfig.Spec.Build.BaseImage {

	// for backwards compatibility
	case "", "alpine":
		if p.FunctionConfig.Spec.Runtime == "python:2.7" {
			baseImage = "python:2.7-alpine"
		} else {
			baseImage = "python:3.6-alpine"
		}

	// for backwards compatibility
	case "jessie":
		if p.FunctionConfig.Spec.Runtime == "python:2.7" {
			baseImage = "python:2.7-jessie"
		} else {
			baseImage = "python:3.6-jessie"
		}

	// if user specified something - use that
	default:
		baseImage = p.FunctionConfig.Spec.Build.BaseImage
	}

	buildArgs["NUCLIO_BASE_IMAGE"] = baseImage

	return buildArgs, nil
}

func (p *python) getFunctionHandler() string {

	// use the function path: /some/path/func.py -> func
	functionFileName := path.Base(p.FunctionConfig.Spec.Build.Path)
	functionFileName = functionFileName[:len(functionFileName)-len(path.Ext(functionFileName))]

	// take that file name without extension and add a default "handler"
	// TODO: parse the python sources for this
	return fmt.Sprintf("%s:%s", functionFileName, "handler")
}
