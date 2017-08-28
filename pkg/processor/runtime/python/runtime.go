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
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/pkg/errors"
)

// TODO: Find a better place (both on file system and configuration)
const (
	socketPath        = "/tmp/nuclio-py.sock"
	connectionTimeout = 10 * time.Second
	eventTimeout      = 10 * time.Second
)

type python struct {
	runtime.AbstractRuntime

	configuration *Configuration
	eventEncoder  *EventJSONEncoder
	outReader     *bufio.Reader
}

// NewRuntime returns a new Python runtime
func NewRuntime(parentLogger nuclio.Logger, configuration *Configuration) (runtime.Runtime, error) {
	logger := parentLogger.GetChild("python").(nuclio.Logger)

	var err error

	abstractRuntime, err := runtime.NewAbstractRuntime(logger, &configuration.Configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Can't create AbstractRuntime")
	}

	// create the command string
	newPythonRuntime := &python{
		AbstractRuntime: *abstractRuntime,
		configuration:   configuration,
	}

	listener, err := newPythonRuntime.createListener()
	if err != nil {
		return nil, errors.Wrapf(err, "Can't listen on %q", socketPath)
	}

	if err = newPythonRuntime.runWrapper(); err != nil {
		return nil, errors.Wrap(err, "Can't run wrapper")
	}

	connChan := make(chan net.Conn)
	errChan := make(chan error)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			errChan <- err
			return
		}
		connChan <- conn
	}()

	select {
	case conn := <-connChan:
		newPythonRuntime.eventEncoder = NewEventJSONEncoder(newPythonRuntime.Logger, conn)
		newPythonRuntime.outReader = bufio.NewReader(conn)
	case err := <-errChan:
		return nil, errors.Wrap(err, "Error getting wrapper connection")
	case <-time.After(connectionTimeout):
		return nil, fmt.Errorf("No connection from wrapper after %s", connectionTimeout)
	}

	return newPythonRuntime, nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// TODO: Move to pkg/util/ somewhere (we have the same code in nubuild)
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (py *python) createListener() (net.Listener, error) {
	if exists(socketPath) {
		if err := os.Remove(socketPath); err != nil {
			return nil, errors.Wrapf(err, "Can't remove socket at %q", socketPath)
		}
	}
	return net.Listen("unix", socketPath)
}

func (py *python) runWrapper() error {
	wrapperScriptPath := py.getWrapperScriptPath()
	py.Logger.InfoWith("Python wrapper script path", "path", wrapperScriptPath)
	if !isFile(wrapperScriptPath) {
		return fmt.Errorf("Can't find wrapper at %q", wrapperScriptPath)
	}

	entryPoint := py.getEntryPoint()
	py.Logger.InfoWith("Python entry point", "entry_point", entryPoint)

	pythonExe, err := py.getPythonExe()
	if err != nil {
		py.Logger.ErrorWith("Can't find Python exe", "error", err)
		return err
	}
	py.Logger.InfoWith("Python executable", "path", pythonExe)

	env := py.getEnvFromConfiguration()
	envPath := fmt.Sprintf("PYTHONPATH=%s", py.getPythonPath())
	py.Logger.InfoWith("PYTHONPATH", "value", envPath)
	env = append(env, envPath)

	args := []string{
		pythonExe, wrapperScriptPath,
		"--entry-point", entryPoint,
		"--socket-path", socketPath,
	}
	py.Logger.InfoWith("Running wrapper", "command", strings.Join(args, " "))
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	out, err := os.Create("/tmp/nuclio-py.log")
	if err == nil {
		cmd.Stdout = out
		cmd.Stderr = out
	}
	return cmd.Start()
}

func (py *python) handleEvent(event nuclio.Event, outChan chan interface{}, errChan chan error) {
	// Send event
	if err := py.eventEncoder.Encode(event); err != nil {
		errChan <- err
		return
	}

	// Read logs & output
	for {
		data, err := py.outReader.ReadBytes('\n')
		if err != nil {
			errChan <- err
			return
		}

		obj := make(map[string]interface{})
		if err := json.Unmarshal(data, &obj); err != nil {
			errChan <- err
			return
		}

		val, ok := obj["handler_output"]
		if ok {
			outChan <- val
			return
		}

		py.pythonLog(obj)
	}
}

func (py *python) pythonLog(log map[string]interface{}) {
	vars := make([]interface{}, 2*len(log))
	i := 0
	for key, value := range log {
		vars[i] = key
		vars[i+1] = value
		i += 2
	}

	format := "Python Log"
	switch log["levelname"] {
	case "ERROR", "CRITICAL":
		py.Logger.ErrorWith(format, vars...)
	case "WARNING":
		py.Logger.WarnWith(format, vars...)
	case "INFO":
		py.Logger.InfoWith(format, vars...)
	default:
		py.Logger.DebugWith(format, vars...)
	}
}

func (py *python) ProcessEvent(event nuclio.Event) (interface{}, error) {
	py.Logger.DebugWith("Executing python",
		"name", py.configuration.Name,
		"version", py.configuration.Version,
		"eventID", event.GetID())

	outChan, errChan := make(chan interface{}), make(chan error)
	go py.handleEvent(event, outChan, errChan)
	select {
	case out := <-outChan:
		py.Logger.DebugWith("python executed",
			"out", out,
			"eventID", event.GetID())
		return out, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(eventTimeout):
		return nil, fmt.Errorf("handler timeout after %s", eventTimeout)
	}
}

func (py *python) getEnvFromConfiguration() []string {
	return []string{
		fmt.Sprintf("NUCLIO_FUNCTION_NAME=%s", py.configuration.Name),
		fmt.Sprintf("NUCLIO_FUNCTION_DESCRIPTION=%s", py.configuration.Description),
		fmt.Sprintf("NUCLIO_FUNCTION_VERSION=%s", py.configuration.Version),
	}
}

func (py *python) getEnvFromEvent(event nuclio.Event) []string {
	return []string{
		fmt.Sprintf("NUCLIO_EVENT_ID=%s", event.GetID()),
		fmt.Sprintf("NUCLIO_EVENT_SOURCE_CLASS=%s", event.GetSource().GetClass()),
		fmt.Sprintf("NUCLIO_EVENT_SOURCE_KIND=%s", event.GetSource().GetKind()),
	}
}

func (py *python) getEntryPoint() string {
	return py.configuration.EntryPoint
}

// TODO: Global processor configuration, where should this go?
func (py *python) getWrapperScriptPath() string {
	scriptPath := os.Getenv("NUCLIO_PYTHON_WRAPPER")
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

func (py *python) getPythonExe() (string, error) {
	baseName := "python3"
	if py.configuration.PythonVersion == "2" {
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
