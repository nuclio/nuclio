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

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/util/common"

	"github.com/nuclio/nuclio-sdk"
	"github.com/rs/xid"
)

// TODO: Find a better place (both on file system and configuration)
const (
	socketPathTemplate = "/tmp/nuclio-py-%s.sock"
	connectionTimeout  = 10 * time.Second
	eventTimeout       = 10 * time.Second
)

type result struct {
	StatusCode  int    `json:"status_code"`
	ContentType string `json:"content_type"`
	Body        string `json:"body"`
	err         error
}

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
		return nil, errors.Wrapf(err, "Can't listen on %q", newPythonRuntime.socketPath())
	}

	if err = newPythonRuntime.runWrapper(); err != nil {
		return nil, errors.Wrap(err, "Can't run wrapper")
	}

	unixListener, ok := listener.(*net.UnixListener)
	if !ok {
		return nil, errors.Wrap(err, "Can't get underlying Unix listener")
	}
	if err := unixListener.SetDeadline(time.Now().Add(connectionTimeout)); err != nil {
		return nil, errors.Wrap(err, "Can't set deadline")
	}
	conn, err := listener.Accept()
	if err != nil {
		return nil, errors.Wrap(err, "Can't get connection from Python wrapper")
	}

	newPythonRuntime.eventEncoder = NewEventJSONEncoder(newPythonRuntime.Logger, conn)
	newPythonRuntime.outReader = bufio.NewReader(conn)

	return newPythonRuntime, nil
}

func (py *python) ProcessEvent(event nuclio.Event, functionLogger nuclio.Logger) (interface{}, error) {
	py.Logger.DebugWith("Processing event",
		"name", py.configuration.Name,
		"version", py.configuration.Version,
		"eventID", event.GetID())

	resultChan := make(chan *result)
	go py.handleEvent(functionLogger, event, resultChan)

	select {
	case result := <-resultChan:
		py.Logger.DebugWith("Python executed",
			"result", result,
			"eventID", event.GetID())
		return nuclio.Response{
			StatusCode:  result.StatusCode,
			ContentType: result.ContentType,
			Body:        []byte(result.Body),
		}, nil
	case <-time.After(eventTimeout):
		return nil, fmt.Errorf("handler timeout after %s", eventTimeout)
	}
}

func (py *python) createListener() (net.Listener, error) {
	socketPath := py.socketPath()
	if common.FileExists(socketPath) {
		if err := os.Remove(socketPath); err != nil {
			return nil, errors.Wrapf(err, "Can't remove socket at %q", socketPath)
		}
	}
	return net.Listen("unix", socketPath)
}

func (py *python) socketPath() string {
	return fmt.Sprintf(socketPathTemplate, xid.New().String())
}

func (py *python) runWrapper() error {
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

	env := py.getEnvFromConfiguration()
	envPath := fmt.Sprintf("PYTHONPATH=%s", py.getPythonPath())
	py.Logger.DebugWith("Setting PYTHONPATH", "value", envPath)
	env = append(env, envPath)

	args := []string{
		pythonExePath, wrapperScriptPath,
		"--handler", handler,
		"--socket-path", py.socketPath(),
	}

	py.Logger.DebugWith("Running wrapper", "command", strings.Join(args, " "))

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	return cmd.Start()
}

func (py *python) handleEvent(functionLogger nuclio.Logger, event nuclio.Event, resultChan chan *result) {
	unmarshalledResult := &result{}

	// Send event
	if unmarshalledResult.err = py.eventEncoder.Encode(event); unmarshalledResult.err != nil {
		resultChan <- unmarshalledResult
		return
	}

	var data []byte

	// Read logs & output
	for {
		data, unmarshalledResult.err = py.outReader.ReadBytes('\n')

		if unmarshalledResult.err != nil {
			py.Logger.WarnWith("Failed to read from connection", "err", unmarshalledResult.err)

			resultChan <- unmarshalledResult
			return
		}

		switch data[0] {
		case 'r':

			// try to unmarshall the result
			if unmarshalledResult.err = json.Unmarshal(data[1:], unmarshalledResult); unmarshalledResult.err != nil {
				resultChan <- unmarshalledResult
				return
			}

			// write back to result channel
			resultChan <- unmarshalledResult

			return

		case 'l':
			py.handleResponseLog(functionLogger, data[1:])
		}
	}
}

func (py *python) handleResponseLog(functionLogger nuclio.Logger, response []byte) {
	log := make(map[string]interface{})

	if err := json.Unmarshal(response, &log); err != nil {
		return
	}

	message, levelName := log["message"], log["level"]

	for _, fieldName := range []string{"message", "level", "datetime"} {
		delete(log, fieldName)
	}

	vars := make([]interface{}, 2*len(log))
	i := 0
	for key, value := range log {
		vars[i] = key
		vars[i+1] = value
		i += 2
	}

	// if we got a per-invocation logger, use that. otherwise use the root logger for functions
	logger := functionLogger
	if logger == nil {
		logger = py.FunctionLogger
	}

	logFunc := logger.DebugWith

	switch levelName {
	case "error", "critical":
		logFunc = logger.ErrorWith
	case "warning":
		logFunc = logger.WarnWith
	case "info":
		logFunc = logger.InfoWith
	}

	logFunc(message, vars...)
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

func (py *python) getHandler() string {
	return py.configuration.Handler
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
