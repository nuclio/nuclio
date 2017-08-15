package shell

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/pkg/errors"
)

type shell struct {
	runtime.AbstractRuntime
	configuration *Configuration
	command       string
	env           []string
	ctx           context.Context
}

func NewRuntime(parentLogger nuclio.Logger, configuration *Configuration) (runtime.Runtime, error) {

	// create the command string
	newShellRuntime := &shell{
		AbstractRuntime: *runtime.NewAbstractRuntime(parentLogger.GetChild("shell").(nuclio.Logger), &configuration.Configuration),
		ctx:             context.Background(),
		configuration:   configuration,
	}

	// update it with some stuff so that we don't have to do this each invocation
	newShellRuntime.command = newShellRuntime.getCommandString()
	newShellRuntime.env = newShellRuntime.getEnvFromConfiguration()

	return newShellRuntime, nil
}

func (s *shell) ProcessEvent(event nuclio.Event) (interface{}, error) {
	s.Logger.DebugWith("Executing shell",
		"name", s.configuration.Name,
		"version", s.configuration.Version,
		"eventID", event.GetID())

	// create a timeout context
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	// create a command
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", s.command+" "+event.GetContentType())
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
		"out", string(out),
		"eventID", event.GetID())

	return out, nil
}

func (s *shell) getCommandString() string {
	command := s.configuration.ScriptPath + " "
	command += strings.Join(s.configuration.ScriptArgs, " ")

	return command
}

func (s *shell) getEnvFromConfiguration() []string {
	return []string{
		fmt.Sprintf("NUCLIO_FUNCTION_NAME=%s", s.configuration.Name),
		fmt.Sprintf("NUCLIO_FUNCTION_DESCRIPTION=%s", s.configuration.Description),
		fmt.Sprintf("NUCLIO_FUNCTION_VERSION=%s", s.configuration.Version),
	}
}

func (s *shell) getEnvFromEvent(event nuclio.Event) []string {
	return []string{
		fmt.Sprintf("NUCLIO_EVENT_ID=%s", event.GetID()),
		fmt.Sprintf("NUCLIO_EVENT_SOURCE_CLASS=%s", event.GetSource().GetClass()),
		fmt.Sprintf("NUCLIO_EVENT_SOURCE_KIND=%s", event.GetSource().GetKind()),
	}
}
