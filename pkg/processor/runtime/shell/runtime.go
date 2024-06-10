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

package shell

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type shell struct {
	*runtime.AbstractRuntime
	configuration  *Configuration
	command        string
	env            []string
	commandInPath  bool
	ctx            context.Context
	restartChannel chan struct{}
}

// NewRuntime returns a new shell runtime
func NewRuntime(parentLogger logger.Logger, configuration *Configuration) (runtime.Runtime, error) {
	runtimeLogger := parentLogger.GetChild("shell")

	// create base
	abstractRuntime, err := runtime.NewAbstractRuntime(runtimeLogger, configuration.Configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract runtime")
	}

	// create the command string
	newShellRuntime := &shell{
		AbstractRuntime: abstractRuntime,
		ctx:             context.Background(),
		configuration:   configuration,
	}

	// update it with some stuff so that we don't have to do this each invocation
	newShellRuntime.command, err = newShellRuntime.getCommand()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get command")
	}

	newShellRuntime.env = newShellRuntime.getEnvFromConfiguration()
	newShellRuntime.restartChannel = make(chan struct{}, 1)

	newShellRuntime.commandInPath, err = newShellRuntime.commandIsInPath()
	if err != nil {
		newShellRuntime.Logger.ErrorWith("Failed checking if command is in PATH",
			"name", newShellRuntime.configuration.Meta.Name,
			"version", newShellRuntime.configuration.Spec.Version,
			"command", newShellRuntime.command,
			"err", err)
		return nil, errors.Wrap(err, "Failed checking if command is in PATH")
	}

	newShellRuntime.SetStatus(status.Ready)

	return newShellRuntime, nil
}

func (s *shell) ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (interface{}, error) {
	command := []string{s.command}
	command = append(command, s.getCommandArguments(event)...)

	// create a timeout context
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	s.Logger.DebugWith("Executing shell",
		"name", s.configuration.Meta.Name,
		"eventID", event.GetID(),
		"bodyLen", len(event.GetBody()),
		"command", command,
		"eventTimeout", s.configuration.Spec.EventTimeout)

	responseChan := make(chan nuclio.Response, 1)

	// process event in background
	go s.processEvent(ctx, command, event, responseChan)

	// wait for event response, return once it is done (or errored)
	for {
		select {
		case response := <-responseChan:
			return response, nil

		case <-ctx.Done():
			return nil, nuclio.NewErrRequestTimeout("Failed waiting for function execution")

		case <-s.restartChannel:
			s.Logger.Warn("Cancelling execution due to an ongoing restart")
			cancel()
		}
	}
}

func (s *shell) ProcessBatch(batch []nuclio.Event, functionLogger logger.Logger) ([]*runtime.ResponseWithErrors, error) {
	return nil, nuclio.ErrNotImplemented
}

func (s *shell) processEvent(context context.Context,
	command []string,
	event nuclio.Event,
	responseChan chan nuclio.Response) {

	response := nuclio.Response{
		StatusCode: http.StatusInternalServerError,
		Headers:    s.configuration.ResponseHeaders,
	}

	// write response upon finishing
	defer func() {
		responseChan <- response
	}()

	var cmd *exec.Cmd

	if s.commandInPath {

		// if the command is an executable, run it as a command with sh -c.
		cmd = exec.CommandContext(context, "sh", "-c", strings.Join(command, " "))
	} else {

		// if the command is a shell script run it with sh(without -c). this will make sh
		// read the script and run it as shell script and run it.
		cmd = exec.CommandContext(context, "sh", command...)
	}

	cmd.Stdin = strings.NewReader(string(event.GetBody()))

	// set the command env
	cmd.Env = s.env

	// add event stuff to env
	cmd.Env = append(cmd.Env, s.getEnvFromEvent(event)...)

	// save timestamp
	startTime := time.Now()

	// run the command
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.Logger.ErrorWith("Failed to run shell command",
			"name", s.configuration.Meta.Name,
			"version", s.configuration.Spec.Version,
			"eventID", event.GetID(),
			"bodyLen", len(event.GetBody()),
			"command", command,
			"err", err)
		response.Body = []byte(fmt.Sprintf(ResponseErrorFormat, err, out))
		return
	}

	// calculate call duration
	callDuration := time.Since(startTime)

	// add duration to sum
	s.Statistics.DurationMilliSecondsSum += uint64(callDuration.Nanoseconds() / 1000000)
	s.Statistics.DurationMilliSecondsCount++

	s.Logger.DebugWith("Shell executed",
		"eventID", event.GetID(),
		"callDuration", callDuration)
	response.StatusCode = http.StatusOK
	response.Body = out
}

func (s *shell) getCommand() (string, error) {
	var command string

	moduleName, entrypoint, err := functionconfig.ParseHandler(s.configuration.Spec.Handler)
	if err != nil {
		return "", errors.Wrap(err, "Failed to parse handler")
	}

	// if there's only one segment in the handler, in shell's case, it's the module name
	if moduleName == "" {
		moduleName = entrypoint
	}

	// if there's a directory passed as an environment telling us where to look for the module, use it. otherwise
	// use /opt/nuclio
	shellHandlerDir := os.Getenv("NUCLIO_SHELL_HANDLER_DIR")
	if shellHandlerDir == "" {
		shellHandlerDir = "/opt/nuclio/"
	}

	shellHandlerPath := path.Join(shellHandlerDir, moduleName)

	// is there really a file there? could be user set module to something on the path
	if common.FileExists(shellHandlerPath) {

		command = shellHandlerPath
	} else {

		// the command is simply the module name
		command = moduleName
	}

	return command, nil
}

func (s *shell) getCommandArguments(event nuclio.Event) []string {
	arguments := event.GetHeaderString(headers.Arguments)

	if arguments == "" {
		arguments = s.configuration.Arguments
	}

	return strings.Split(arguments, " ")
}

func (s *shell) getEnvFromConfiguration() []string {
	envs := s.AbstractRuntime.GetEnvFromConfiguration()

	// inject all environment variables passed in configuration
	for _, configEnv := range s.configuration.Spec.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", configEnv.Name, configEnv.Value))
	}

	return envs
}

func (s *shell) getEnvFromEvent(event nuclio.Event) []string {
	return []string{
		fmt.Sprintf("NUCLIO_EVENT_ID=%s", event.GetID()),
		fmt.Sprintf("NUCLIO_TRIGGER_CLASS=%s", event.GetTriggerInfo().GetClass()),
		fmt.Sprintf("NUCLIO_TRIGGER_KIND=%s", event.GetTriggerInfo().GetKind()),
		fmt.Sprintf("NUCLIO_EVENT_CONTENT_TYPE=%s", event.GetContentType()),
		fmt.Sprintf("NUCLIO_EVENT_TIMESTAMP=%s", event.GetTimestamp().UTC().Format(time.RFC3339)),
		fmt.Sprintf("NUCLIO_EVENT_PATH=%s", event.GetPath()),
		fmt.Sprintf("NUCLIO_EVENT_URL=%s", event.GetURL()),
		fmt.Sprintf("NUCLIO_EVENT_METHOD=%s", event.GetMethod()),
		fmt.Sprintf("NUCLIO_EVENT_SHARD_ID=%d", event.GetShardID()),
		fmt.Sprintf("NUCLIO_EVENT_NUM_SHARDS=%d", event.GetTotalNumShards()),
		fmt.Sprintf("NUCLIO_EVENT_TYPE=%s", event.GetType()),
		fmt.Sprintf("NUCLIO_EVENT_TYPE_VERSION=%s", event.GetTypeVersion()),
		fmt.Sprintf("NUCLIO_EVENT_VERSION=%s", event.GetVersion()),
	}
}

func (s *shell) Restart() error {
	if err := s.Stop(); err != nil {
		return errors.Wrap(err, "Failed to stop runtime")
	}
	s.Logger.Warn("Restarting")
	s.restartChannel <- struct{}{}
	return s.Start()
}

func (s *shell) Start() error {
	s.SetStatus(status.Ready)
	return nil
}

func (s *shell) SupportsRestart() bool {
	return true
}

func (s *shell) commandIsInPath() (bool, error) {

	// Checks if the command is in path, or it's file exists locally
	if !common.FileExists(s.command) {

		// file doesn't exist, checking PATH
		if _, err := exec.LookPath(s.command); err != nil {
			return false, errors.Wrap(err, "File doesn't exist neither in working dir nor in PATH")
		}

		// file is in PATH, we consider this a command whether it is an executable or not
		return true, nil
	}

	return false, nil
}
