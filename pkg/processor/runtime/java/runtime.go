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
	"fmt"
	"os"
	"os/exec"
	"path"
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

func (j *java) runWrapper(port string) error {

	handlerJar, handlerName, err := j.parseHandler()
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"java",
		"-jar", j.wrapperJarPath(),
		"-handler", handlerName,
		"-jar", handlerJar,
		"-port", port,
	)

	j.Logger.InfoWith("Running wrapper jar", "command", strings.Join(cmd.Args, " "))

	return cmd.Start()
}

func (j *java) wrapperJarPath() string {
	wrapperPath := os.Getenv("NUCLIO_WRAPPER_JAR")
	if wrapperPath != "" {
		return wrapperPath
	}

	return "/opt/nuclio/nuclio-java-wrapper.jar"
}

func (j *java) parseHandler() (string, string, error) {
	parts := strings.Split(j.configuration.Spec.Handler, ":")

	jarFileName := "handler.jar"
	handlerName := ""

	switch len(parts) {
	case 1:
		handlerName = parts[0]
	case 2:
		jarFileName = parts[0]
		handlerName = parts[1]
	default:
		return "", "", fmt.Errorf("Bad handler - %q", j.configuration.Spec.Handler)
	}

	return path.Join(j.getHandlerDir(), jarFileName), handlerName, nil
}

func (j *java) getHandlerDir() string {
	handlerDir := os.Getenv("NUCLIO_HANDLER_DIR")
	if handlerDir != "" {
		return handlerDir
	}

	return "/opt/nuclio/handler"
}
