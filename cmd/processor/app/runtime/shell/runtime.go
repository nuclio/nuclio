package shell

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/runtime"
	"github.com/nuclio/nuclio/pkg/logger"
)

type shell struct {
	runtime.AbstractRuntime
	configuration *Configuration
	command       string
	env           []string
	ctx           context.Context
}

func NewRuntime(logger logger.Logger, configuration *Configuration) (runtime.Runtime, error) {

	// create the command string
	newShellRuntime := &shell{
		AbstractRuntime: *runtime.NewAbstractRuntime(logger.GetChild("shell"), &configuration.Configuration),
		ctx:             context.Background(),
		configuration:   configuration,
	}

	// update it with some stuff so that we don't have to do this each invocation
	newShellRuntime.command = newShellRuntime.getCommandString()
	newShellRuntime.env = newShellRuntime.getEnvFromConfiguration()

	return newShellRuntime, nil
}

func (s *shell) ProcessEvent(event event.Event) (interface{}, error) {
	s.Logger.With(logger.Fields{
		"name":    s.configuration.Name,
		"version": s.configuration.Version,
		"eventID": *event.GetID(),
	}).Debug("Executing shell")

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

	s.Logger.With(logger.Fields{
		"out":     string(out),
		"eventID": *event.GetID(),
	}).Debug("Shell executed")

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

func (s *shell) getEnvFromEvent(event event.Event) []string {
	return []string{
		fmt.Sprintf("NUCLIO_EVENT_ID=%s", *event.GetID()),
		fmt.Sprintf("NUCLIO_EVENT_SOURCE_CLASS=%s", event.GetSource().GetClass()),
		fmt.Sprintf("NUCLIO_EVENT_SOURCE_KIND=%s", event.GetSource().GetKind()),
	}
}
