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
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/status"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type shell struct {
	*runtime.AbstractRuntime
	configuration *Configuration
	command       string
	env           []string
	ctx           context.Context
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

	newShellRuntime.SetStatus(status.Ready)

	return newShellRuntime, nil
}

func (s *shell) ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (interface{}, error) {
	command := s.command

	command += " " + s.getCommandArguments(event)

	s.Logger.DebugWith("Executing shell",
		"name", s.configuration.Meta.Name,
		"version", s.configuration.Spec.Version,
		"eventID", event.GetID(),
		"bodyLen", len(event.GetBody()),
		"command", command)

	// create a timeout context (TODO: from configuration)
	ctx, cancel := context.WithTimeout(s.ctx, 60*time.Second)
	defer cancel()

	// create a command
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stdin = strings.NewReader(string(event.GetBody()))

	// set the command env
	cmd.Env = s.env

	// add event stuff to env
	cmd.Env = append(cmd.Env, s.getEnvFromEvent(event)...)

	// run the command
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to run shell command")
	}

	s.Logger.DebugWith("Shell executed",
		"eventID", event.GetID())

	return nuclio.Response{
		StatusCode: http.StatusOK,
		Headers:    s.configuration.ResponseHeaders,
		Body:       out,
	}, nil
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

		// set permissions of handler such that if it wasn't executable before, it's executable now
		if err := os.Chmod(shellHandlerPath, 0755); err != nil {
			return "", errors.Wrapf(err, "Failed to change mode for %s", shellHandlerPath)
		}

		command = shellHandlerPath
	} else {

		// the command is simply the module name
		command = moduleName
	}

	return command, nil
}

func (s *shell) getCommandArguments(event nuclio.Event) string {
	if arguments := event.GetHeaderString("x-nuclio-arguments"); arguments != "" {
		return arguments
	}

	return s.configuration.Arguments
}

func (s *shell) getEnvFromConfiguration() []string {
	envs := []string{
		fmt.Sprintf("NUCLIO_FUNCTION_NAME=%s", s.configuration.Meta.Name),
		fmt.Sprintf("NUCLIO_FUNCTION_DESCRIPTION=%s", s.configuration.Spec.Description),
		fmt.Sprintf("NUCLIO_FUNCTION_VERSION=%d", s.configuration.Spec.Version),
	}

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
