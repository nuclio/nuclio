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
	"os"
	"os/exec"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc"

	"github.com/nuclio/nuclio-sdk"
)

type python struct {
	*rpc.Runtime
	Logger        nuclio.Logger
	configuration *runtime.Configuration
}

// NewRuntime returns a new Python runtime
func NewRuntime(parentLogger nuclio.Logger, configuration *runtime.Configuration) (runtime.Runtime, error) {
	newPythonRuntime := &python{
		configuration: configuration,
		Logger:        parentLogger.GetChild("logger"),
	}

	var err error
	newPythonRuntime.Runtime, err = rpc.NewRPCRuntime(newPythonRuntime.Logger, configuration, newPythonRuntime.runWrapper)

	return newPythonRuntime, err
}

func (py *python) runWrapper(socketPath string) error {
	wrapperScriptPath := py.getWrapperScriptPath()
	py.Logger.DebugWith("Using Python wrapper script path", "path", wrapperScriptPath)
	if !common.IsFile(wrapperScriptPath) {
		return fmt.Errorf("Can't find wrapper at %q", wrapperScriptPath)
	}

	handler := py.getHandler()
	py.Logger.DebugWith("Using Python handler", "handler", handler)

	pythonExePath, err := py.getPythonExePath()
	if err != nil {
		py.Logger.ErrorWith("Can't find Python exe", "error", err)
		return err
	}
	py.Logger.DebugWith("Using Python executable", "path", pythonExePath)

	// pass global environment onto the process, and sprinkle in some added env vars
	env := os.Environ()
	env = append(env, py.getEnvFromConfiguration()...)
	envPath := fmt.Sprintf("PYTHONPATH=%s", py.getPythonPath())
	py.Logger.DebugWith("Setting PYTHONPATH", "value", envPath)
	env = append(env, envPath)

	args := []string{
		pythonExePath, wrapperScriptPath,
		"--handler", handler,
		"--socket-path", socketPath,
	}

	py.Logger.DebugWith("Running wrapper", "command", strings.Join(args, " "))

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	return cmd.Start()
}

func (py *python) getEnvFromConfiguration() []string {
	return []string{
		fmt.Sprintf("NUCLIO_FUNCTION_NAME=%s", py.configuration.Meta.Name),
		fmt.Sprintf("NUCLIO_FUNCTION_DESCRIPTION=%s", py.configuration.Spec.Description),
		fmt.Sprintf("NUCLIO_FUNCTION_VERSION=%d", py.configuration.Spec.Version),
	}
}

func (py *python) getHandler() string {
	return py.configuration.Spec.Handler
}

// TODO: Global processor configuration, where should this go?
func (py *python) getWrapperScriptPath() string {
	scriptPath := os.Getenv("NUCLIO_PYTHON_WRAPPER_PATH")
	if len(scriptPath) == 0 {
		return "/opt/nuclio/wrapper.py"
	}

	return scriptPath
}

func (py *python) getPythonPath() string {
	pythonPath := os.Getenv("NUCLIO_PYTHON_PATH")
	if len(pythonPath) == 0 {
		return "/opt/nuclio"
	}

	return pythonPath
}

func (py *python) getPythonExePath() (string, error) {
	baseName := "python3"

	_, runtimeVersion := py.configuration.Spec.GetRuntimeNameAndVersion()

	if strings.HasPrefix(runtimeVersion, "2") {
		baseName = "python2"
	}

	exePath, err := exec.LookPath(baseName)
	if err == nil {
		return exePath, nil
	}

	py.Logger.WarnWith("Can't find specific python exe", "name", baseName)

	// Try just "python"
	exePath, err = exec.LookPath("python")
	if err == nil {
		return exePath, nil
	}

	return "", errors.Wrap(err, "Can't find python executable")
}
