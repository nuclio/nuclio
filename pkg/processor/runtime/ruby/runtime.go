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
	"os"
	"os/exec"
	"strings"

	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc"

	"github.com/nuclio/logger"
)

type ruby struct {
	*rpc.Runtime
	Logger        logger.Logger
	configuration *runtime.Configuration
}

// NewRuntime returns a new Ruby runtime
func NewRuntime(parentLogger logger.Logger, configuration *runtime.Configuration) (runtime.Runtime, error) {

	newJavaRuntime := &ruby{
		configuration: configuration,
		Logger:        parentLogger.GetChild("logger"),
	}

	var err error
	newJavaRuntime.Runtime, err = rpc.NewRPCRuntime(newJavaRuntime.Logger, configuration, newJavaRuntime.runWrapper, rpc.UnixSocket)

	return newJavaRuntime, err
}

func (r *ruby) runWrapper(socketPath string) (*os.Process, error) {
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
