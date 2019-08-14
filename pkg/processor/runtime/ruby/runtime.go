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

package ruby

import (
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc"

	"github.com/nuclio/logger"
)

type ruby struct {
	*rpc.AbstractRuntime
	Logger        logger.Logger
	configuration *runtime.Configuration
}

// NewRuntime returns a new Ruby runtime
func NewRuntime(parentLogger logger.Logger, configuration *runtime.Configuration) (runtime.Runtime, error) {
	var err error

	newRubyRuntime := &ruby{
		configuration: configuration,
		Logger:        parentLogger.GetChild("logger"),
	}

	newRubyRuntime.AbstractRuntime, err = rpc.NewAbstractRuntime(newRubyRuntime.Logger,
		configuration,
		newRubyRuntime)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create runtime")
	}

	return newRubyRuntime, nil
}

func (r *ruby) RunWrapper(socketPath string) (*os.Process, error) {
	args := []string{
		"ruby",
		"/opt/nuclio/wrapper.rb",
		"--handler", r.configuration.Spec.Handler,
		"--socket-path", socketPath,
	}

	cmd := exec.Command(args[0], args[1:]...)
	r.Logger.InfoWith("Running ruby wrapper", "command", strings.Join(cmd.Args, " "))

	return cmd.Process, cmd.Start()
}

func (r *ruby) GetEventEncoder(writer io.Writer) rpc.EventEncoder {
	return rpc.NewEventJSONEncoder(r.Logger, writer)
}
