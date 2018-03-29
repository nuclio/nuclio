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

package java

import (
	"os"
	"os/exec"
	"strings"

	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc"

	"github.com/nuclio/logger"
)

type java struct {
	*rpc.Runtime
	Logger        logger.Logger
	configuration *runtime.Configuration
}

// NewRuntime returns a new Java runtime
func NewRuntime(parentLogger logger.Logger, configuration *runtime.Configuration) (runtime.Runtime, error) {

	newJavaRuntime := &java{
		configuration: configuration,
		Logger:        parentLogger.GetChild("logger"),
	}

	var err error
	newJavaRuntime.Runtime, err = rpc.NewRPCRuntime(newJavaRuntime.Logger, configuration, newJavaRuntime.runWrapper, rpc.TCPSocket)

	return newJavaRuntime, err
}

func (j *java) runWrapper(port string) (*os.Process, error) {
	cmd := exec.Command(
		"java",
		"-jar", j.wrapperJarPath(),
		"-handler", j.handlerName(),
		"-port", port,
	)

	j.Logger.InfoWith("Running wrapper jar", "command", strings.Join(cmd.Args, " "))

	return cmd.Process, cmd.Start()
}

func (j *java) wrapperJarPath() string {
	wrapperPath := os.Getenv("NUCLIO_WRAPPER_JAR")
	if wrapperPath != "" {
		return wrapperPath
	}

	return "/opt/nuclio/nuclio-java-wrapper.jar"
}

func (j *java) handlerName() string {
	if !strings.Contains(j.configuration.Spec.Handler, ":") {
		return j.configuration.Spec.Handler
	}

	// "module:handler" -> "handler"
	fields := strings.SplitN(j.configuration.Spec.Handler, ":", 2)
	return fields[1]
}
