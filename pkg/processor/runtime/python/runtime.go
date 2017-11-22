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

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/nuclio-sdk"
	"github.com/rs/xid"
)

// TODO: Find a better place (both on file system and configuration)
const (
	socketPathTemplate = "/tmp/nuclio-py-%s.sock"
	connectionTimeout  = 10 * time.Second
	eventTimeout       = 5 * time.Minute
)

type result struct {
	StatusCode  int                    `json:"status_code"`
	ContentType string                 `json:"content_type"`
	Body        string                 `json:"body"`
	Headers     map[string]interface{} `json:"headers"`
	err         error
}

type python struct {
	runtime.AbstractRuntime
	configuration *Configuration
	eventEncoder  *EventJSONEncoder
	outReader     *bufio.Reader
	socketPath    string
}

type pythonLogRecord struct {
	DateTime string                 `json:"datetime"`
	Level    string                 `json:"level"`
	Message  string                 `json:"message"`
	With     map[string]interface{} `json:"with"`
}

// NewRuntime returns a new Python runtime
func NewRuntime(parentLogger nuclio.Logger, configuration *Configuration) (runtime.Runtime, error) {
	logger := parentLogger.GetChild("python")

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

	// create socket path
	newPythonRuntime.socketPath = newPythonRuntime.createSocketPath()

	listener, err := newPythonRuntime.createListener()
	if err != nil {
		return nil, errors.Wrapf(err, "Can't listen on %q", newPythonRuntime.socketPath)
	}

	if err = newPythonRuntime.runWrapper(); err != nil {
		return nil, errors.Wrap(err, "Can't run wrapper")
	}

	unixListener, ok := listener.(*net.UnixListener)
	if !ok {
		return nil, errors.Wrap(err, "Can't get underlying Unix listener")
	}
	if err = unixListener.SetDeadline(time.Now().Add(connectionTimeout)); err != nil {
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
			Body:        []byte(result.Body),
			ContentType: result.ContentType,
			Headers:     result.Headers,
			StatusCode:  result.StatusCode,
		}, nil
	case <-time.After(eventTimeout):
		return nil, fmt.Errorf("handler timeout after %s", eventTimeout)
	}
}

func (py *python) createListener() (net.Listener, error) {
	if common.FileExists(py.socketPath) {
		if err := os.Remove(py.socketPath); err != nil {
			return nil, errors.Wrapf(err, "Can't remove socket at %q", py.socketPath)
		}
	}

	py.Logger.DebugWith("Creating listener socket", "path", py.socketPath)

	return net.Listen("unix", py.socketPath)
}

func (py *python) createSocketPath() string {
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

	// pass global environment onto the process, and sprinkle in some added env vars
	env := os.Environ()
	env = append(env, py.getEnvFromConfiguration()...)
	envPath := fmt.Sprintf("PYTHONPATH=%s", py.getPythonPath())
	py.Logger.DebugWith("Setting PYTHONPATH", "value", envPath)
	env = append(env, envPath)

	args := []string{
		pythonExePath, wrapperScriptPath,
		"--handler", handler,
		"--socket-path", py.socketPath,
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

		case 'm':
			py.handleReponseMetric(functionLogger, data[1:])

		case 'l':
			py.handleResponseLog(functionLogger, data[1:])
		}
	}
}

// {"a": 1, "b": 2} -> ["a", 1, "b", 2]
func (py *python) mapToSlice(m map[string]interface{}) []interface{} {
	slice := make([]interface{}, 0, 2*len(m))

	for key, value := range m {
		slice = append(slice, key)
		slice = append(slice, value)
	}
	return slice
}

func (py *python) handleResponseLog(functionLogger nuclio.Logger, response []byte) {
	var logRecord pythonLogRecord

	if err := json.Unmarshal(response, &logRecord); err != nil {
		py.Logger.ErrorWith("Can't decode log", "error", err)
		return
	}

	logger := py.resolveFunctionLogger(functionLogger)
	logFunc := logger.DebugWith

	switch logRecord.Level {
	case "error", "critical", "fatal":
		logFunc = logger.ErrorWith
	case "warning":
		logFunc = logger.WarnWith
	case "info":
		logFunc = logger.InfoWith
	}

	vars := py.mapToSlice(logRecord.With)
	logFunc(logRecord.Message, vars...)
}

func (py *python) handleReponseMetric(functionLogger nuclio.Logger, response []byte) {
	var metrics struct {
		DurationSec float64 `json:"duration"`
	}

	logger := py.resolveFunctionLogger(functionLogger)
	if err := json.Unmarshal(response, &metrics); err != nil {
		logger.ErrorWith("Can't decode metric", "error", err)
		return
	}

	if metrics.DurationSec == 0 {
		logger.ErrorWith("No duration in metrics", "metrics", metrics)
		return
	}

	py.Statistics.DurationMilliSecondsCount++
	py.Statistics.DurationMilliSecondsSum += uint64(metrics.DurationSec * 1000)
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

// resolveFunctionLogger return either functionLogger if provided or root logger if not
func (py *python) resolveFunctionLogger(functionLogger nuclio.Logger) nuclio.Logger {
	if functionLogger == nil {
		return py.Logger
	}
	return functionLogger
}
