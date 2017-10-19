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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
)

type python struct {
	*runtime.AbstractRuntime
	handlerName string // Cached handler name
}

// returns the image name of the default processor base image
func (p *python) GetDefaultProcessorBaseImageName() string {
	baseImageName := "nuclio/processor-py"

	// make sure the image exists. don't pull if instructed not to
	if !p.Configuration.GetNoBaseImagePull() {
		p.DockerClient.PullImage(baseImageName)
	}

	return baseImageName
}

// given a path holding a function (or functions) returns a list of all the handlers
// in that directory
func (p *python) DetectFunctionHandlers(functionPath string) ([]string, error) {
	handler, err := p.getFunctionHandler()
	return []string{handler}, err
}

func (p *python) GetProcessorConfigFileContents() string {
	// We ignore the error here since we'll fail on parsing before that
	handlerName, _ := p.getFunctionHandler()
	processorConfigFileContentsFormat := `
function:
  kind: "python"
  python_version: "3"
  handler: %s
`
	return fmt.Sprintf(processorConfigFileContentsFormat, handlerName)
}

func (p *python) GetProcessorImageObjectPaths() map[string]string {
	functionPath := p.Configuration.GetFunctionPath()

	if common.IsFile(functionPath) {
		return map[string]string{
			functionPath: path.Join("opt", "nuclio", path.Base(functionPath)),
		}
	}

	return map[string]string{
		functionPath: path.Join("opt", "nuclio"),
	}
}

func (p *python) GetExtension() string {
	return "py"
}

// get the string that signifies a comment if appears at the beginning of the line
func (p *python) GetCommentPattern() string {
	return "#"
}

func (p *python) getFunctionHandler() (string, error) {
	if len(p.handlerName) > 0 {
		return p.handlerName, nil
	}

	module, handler, err := p.parseHandlerSource()
	if err != nil {
		return "", err
	}

	p.handlerName = fmt.Sprintf("%s:%s", module, handler)
	return p.handlerName, nil
}

// parseHandlerSource return module, handler found in Python file
func (p *python) parseHandlerSource() (string, string, error) {
	pythonExePath, err := p.getPythonExePath()
	if err != nil {
		return "", "", err
	}

	scriptPath := p.handlersScriptPath()
	handlerPath := p.Configuration.GetFunctionPath()
	out, err := p.CmdRunner.Run(nil, "%s %s %s", pythonExePath, scriptPath, handlerPath)
	if err != nil {
		return "", "", errors.Wrapf(err, "Can't find handlers in %q", "path", handlerPath)
	}

	// Script emits a JSON array of module, handler objects
	var reply []struct {
		Module  string
		Handler string
	}

	if err := json.Unmarshal([]byte(out), &reply); err != nil {
		return "", "", errors.Wrap(err, "Bad JSON from Python script")
	}

	switch len(reply) {
	case 0:
		return "", "", fmt.Errorf("No handlers found in %q", handlerPath)
	case 1:
		// OK
	default:
		return "", "", fmt.Errorf("Too many handlers found in %q", handlerPath)
	}

	return reply[0].Module, reply[0].Handler, nil
}

func (p *python) handlersScriptPath() string {
	return path.Join(
		p.Configuration.GetNuclioSourceDir(),
		"pkg", "processor", "build", "runtime", "python", "find_handlers.py")
}

// TODO: Unite somehow with the code in runtime
func (p *python) getPythonExePath() (string, error) {
	pythonExePath := os.Getenv("NUCLIO_PYTHON_PATH")
	if len(pythonExePath) > 0 {
		return pythonExePath, nil
	}

	baseName := "python3"
	exePath, err := exec.LookPath(baseName)
	if err == nil {
		return exePath, nil
	}

	p.Logger.WarnWith("Can't find specific python3 exe", "name", baseName)

	// Try just "python"
	exePath, err = exec.LookPath("python")
	if err == nil {
		return exePath, nil
	}

	return "", errors.Wrap(err, "Can't find python executable")
}
