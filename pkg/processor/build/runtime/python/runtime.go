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
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/version"
)

type python struct {
	*runtime.AbstractRuntime
}

// GetProcessorBaseImageName returns the image name of the default processor base image
func (p *python) GetProcessorBaseImageName() (string, error) {

	// get the version we're running so we can pull the compatible image
	versionInfo, err := version.Get()
	if err != nil {
		return "", errors.Wrap(err, "Failed to get version")
	}

	_, runtimeVersion := p.GetRuntimeNameAndVersion()

	// try to get base image name
	baseImageName, err := getBaseImageName(versionInfo,
		runtimeVersion,
		p.FunctionConfig.Spec.Build.BaseImageName)

	if err != nil {
		return "", errors.Wrap(err, "Failed to get base image name")
	}

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
func (p *python) DetectFunctionHandlers(functionPath string) ([]string, error) {
	handler, err := p.getFunctionHandler()
	if err != nil {
		return nil, err
	}
	return []string{handler}, nil
}

// GetProcessorImageObjectPaths returns a map of objects the runtime needs to copy into the processor image
// the key can be a dir, a file or a url of a file
// the value is an absolute path into the docker image
func (p *python) GetProcessorImageObjectPaths() map[string]string {
	functionPath := p.FunctionConfig.Spec.Build.Path

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
func (p *python) GetName() string {
	return "python"
}

func (p *python) getFunctionHandler() (string, error) {
	if len(p.FunctionConfig.Spec.Handler) > 0 {
		return p.FunctionConfig.Spec.Handler, nil
	}

	module, handler, err := p.parseHandlerSource()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s", module, handler), nil
}

func getBaseImageName(versionInfo *version.Info,
	runtimeVersion string,
	baseImageName string) (string, error) {

	// if the runtime version contains any value, use it. otherwise default to 3.6
	if runtimeVersion == "" {
		runtimeVersion = "3.6"
	}

	// if base image name not passed, use alpine
	if baseImageName == "" {
		baseImageName = "alpine"
	}

	// check runtime
	switch runtimeVersion {
	case "2.7", "3.6":
	default:
		return "", fmt.Errorf("Runtime version not supported: %s", runtimeVersion)
	}

	// check base image
	switch baseImageName {
	case "alpine", "jessie":
	default:
		return "", fmt.Errorf("Base image not supported: %s", baseImageName)
	}

	return fmt.Sprintf("nuclio/processor-py%s-%s:%s-%s",
		runtimeVersion,
		baseImageName,
		versionInfo.Label,
		versionInfo.Arch), nil
}

func (p *python) handlersScriptPath() string {
	return path.Join(
		p.FunctionConfig.Spec.Build.NuclioSourceDir,
		"pkg", "processor", "build", "runtime", "python", "find_handlers.py")
}

// parseHandlerSource return module, handler found in Python file
func (p *python) parseHandlerSource() (string, string, error) {
	pythonExePath, err := p.getPythonExePath()
	if err != nil {
		return "", "", err
	}

	scriptPath := fmt.Sprintf("%s/nuclio_find_handlers.py", os.TempDir())
	if err := ioutil.WriteFile(scriptPath, []byte(findHandlerPyCode), 0744); err != nil {
		return "", "", errors.Wrapf(err, "Can't create python file at %s", scriptPath)
	}

	handlerPath := p.FunctionConfig.Spec.Build.Path
	out, err := p.CmdRunner.Run(nil, "%s %s %s", pythonExePath, scriptPath, handlerPath)
	if err != nil {
		return "", "", errors.Wrapf(err, "Can't find handlers in %q", "path", handlerPath)
	}

	// Script emits a JSON array of module, handler objects
	var reply []struct {
		Module  string
		Handler string
	}

	if err := json.Unmarshal([]byte(out.Output), &reply); err != nil {
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
