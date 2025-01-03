/*
Copyright 2023 The Nuclio Authors.

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
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/encoder"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type python struct {
	*rpc.AbstractRuntime
	Logger        logger.Logger
	configuration *runtime.Configuration
}

// NewRuntime returns a new Python runtime
func NewRuntime(parentLogger logger.Logger, configuration *runtime.Configuration) (runtime.Runtime, error) {
	var err error

	newPythonRuntime := &python{
		configuration: configuration,
		Logger:        parentLogger.GetChild("logger"),
	}

	newPythonRuntime.AbstractRuntime, err = rpc.NewAbstractRuntime(newPythonRuntime.Logger,
		configuration,
		newPythonRuntime)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create runtime")
	}

	return newPythonRuntime, nil
}

func (py *python) RunWrapper(eventSocketPaths []string, controlSocketPath string) (*os.Process, error) {

	// TODO: remove warning once python 3.6 is not supported
	_, runtimeVersion := common.GetRuntimeNameAndVersion(py.configuration.Spec.Runtime)
	if runtimeVersion == "" || runtimeVersion == "3.6" {
		py.Logger.Warn("Python 3.6 runtime is deprecated and will soon not be supported. " +
			"Migrate your code and use Python 3.9 runtime (`python:3.9`) or higher")
	}

	wrapperScriptPath := py.getWrapperScriptPath()
	py.Logger.DebugWith("Using Python wrapper script path", "path", wrapperScriptPath)
	if !common.IsFile(wrapperScriptPath) {
		return nil, errors.Errorf("Can't find wrapper at %q", wrapperScriptPath)
	}

	handler := py.getHandler()
	py.Logger.DebugWith("Using Python handler", "handler", handler)

	pythonExePath, err := py.getPythonExePath()
	if err != nil {
		py.Logger.ErrorWith("Can't find Python exe", "error", err)
		return nil, err
	}
	py.Logger.DebugWith("Using Python executable", "path", pythonExePath)

	// pass global environment onto the process, and sprinkle in some added env vars
	env := os.Environ()
	env = append(env, py.AbstractRuntime.GetEnvFromConfiguration()...)
	envPath := fmt.Sprintf("PYTHONPATH=%s", py.getPythonPath())
	py.Logger.DebugWith("Setting PYTHONPATH", "value", envPath)
	env = append(env, envPath)
	eventSocketPathString := strings.Join(eventSocketPaths, ", ")

	args := []string{
		pythonExePath, "-u", wrapperScriptPath,
		"--handler", handler,
		"--event-socket-path", eventSocketPathString,
		"--control-socket-path", controlSocketPath,
		"--platform-kind", py.configuration.PlatformConfig.Kind,
		"--namespace", py.configuration.Meta.Namespace,
		"--worker-id", strconv.Itoa(py.configuration.WorkerID),
		"--trigger-kind", py.configuration.TriggerKind,
		"--trigger-name", py.configuration.TriggerName,
	}

	// whether to decode incoming event messages
	if py.resolveDecodeEvents() {
		args = append(args, "--decode-event-strings")
	}

	py.Logger.DebugWith("Running wrapper", "command", strings.Join(args, " "))

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Process, cmd.Start()
}

// WaitForStart returns whether the runtime supports sending an indication that it started
func (py *python) WaitForStart() bool {
	return true
}

func (py *python) SupportsControlCommunication() bool {
	return true
}

// Drain signals to the runtime to drain its accumulated events and waits for it to finish
func (py *python) Drain() error {

	// do not send a signal if the runtime isn't ready,
	// because the signal handler may not be initialized yet.
	// if the process receives a signal before the handler is set up,
	// the default behaviour will cause the Linux process to terminate.
	if py.AbstractRuntime.GetStatus() != status.Ready {
		return nil
	}

	// we use SIGUSR2 to signal the wrapper process to drain events
	if err := py.AbstractRuntime.Signal(syscall.SIGUSR2); err != nil {
		return errors.Wrap(err, "Failed to signal wrapper process to drain")
	}

	// wait for process to finish event handling or timeout
	// TODO: replace the following function with one that waits for a control communication message or timeout
	py.AbstractRuntime.WaitForProcessTermination(py.configuration.WorkerTerminationTimeout)

	return nil
}

// Terminate signals to the runtime process that processor is about to stop working
func (py *python) Terminate() error {

	// we use SIGUSR1 to signal the wrapper process to terminate
	if err := py.AbstractRuntime.Signal(syscall.SIGUSR1); err != nil {
		return errors.Wrap(err, "Failed to signal wrapper process to terminate")
	}

	// wait for process to finish event handling or timeout
	// TODO: replace the following function with one that waits for a control communication message or timeout
	py.AbstractRuntime.WaitForProcessTermination(py.configuration.WorkerTerminationTimeout)

	return nil
}

// Continue signals the runtime to continue event processing
func (py *python) Continue() error {
	// we use SIGCONT to signal the wrapper process to continue event processing
	if err := py.AbstractRuntime.Signal(syscall.SIGCONT); err != nil {
		return errors.Wrap(err, "Failed to signal wrapper process to continue")
	}

	return nil
}

func (py *python) getHandler() string {
	return py.configuration.Spec.Handler
}

// TODO: Global processor configuration, where should this go?
func (py *python) getWrapperScriptPath() string {
	scriptPath := os.Getenv("NUCLIO_PYTHON_WRAPPER_PATH")
	if len(scriptPath) == 0 {
		return "/opt/nuclio/_nuclio_wrapper.py"
	}

	return scriptPath
}

func (py *python) getPythonPath() string {

	// check if image contains pre-configured PYTHONPATH
	pythonPath := os.Getenv("PYTHONPATH")

	// get user default nuclio python path
	nuclioPythonPath := common.GetEnvOrDefaultString("NUCLIO_PYTHON_PATH", "/opt/nuclio")

	// preserve PYTHONPATH if given, let nuclio come first
	if pythonPath != "" {
		return fmt.Sprintf("%s:%s", nuclioPythonPath, pythonPath)
	}
	return nuclioPythonPath
}

func (py *python) getPythonExePath() (string, error) {

	// let user bring his own python binary
	pythonExePath := os.Getenv("NUCLIO_PYTHON_EXE_PATH")
	if pythonExePath != "" {
		return exec.LookPath(pythonExePath)
	}

	baseName := "python3"

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

func (py *python) GetEventEncoder(writer io.Writer) encoder.EventEncoder {
	return encoder.NewEventMsgPackEncoder(py.Logger, writer)
}

func (py *python) resolveDecodeEvents() bool {

	// switch case for explicitness
	// do not resolve empty or null-able values as false/true for forward/backwards compatibility
	switch strings.ToLower(os.Getenv("NUCLIO_PYTHON_DECODE_EVENT_STRINGS")) {
	case "no", "false", "disable", "disabled":
		return false
	default:
		return true
	}
}
