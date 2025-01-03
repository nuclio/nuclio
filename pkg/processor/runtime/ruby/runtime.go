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

package ruby

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/encoder"

	"github.com/nuclio/errors"
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

func (r *ruby) RunWrapper(socketPaths []string, controlSocketPath string) (*os.Process, error) {
	if len(socketPaths) != 1 {
		return nil, fmt.Errorf("Ruby runtime doesn't support multiple socket processing")
	}
	wrapperPath := common.GetEnvOrDefaultString("NUCLIO_WRAPPER_PATH", "/opt/nuclio/wrapper.rb")
	args := []string{
		"ruby",
		wrapperPath,
		"--handler", r.configuration.Spec.Handler,
		"--socket-path", socketPaths[0],
	}

	env := os.Environ()
	env = append(env, r.AbstractRuntime.GetEnvFromConfiguration()...)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	r.Logger.InfoWith("Running ruby wrapper", "command", strings.Join(cmd.Args, " "))

	return cmd.Process, cmd.Start()
}

func (r *ruby) GetEventEncoder(writer io.Writer) encoder.EventEncoder {
	return encoder.NewEventJSONEncoder(r.Logger, writer)
}
