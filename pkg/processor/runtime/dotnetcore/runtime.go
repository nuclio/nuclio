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

package dotnetcore

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc"

	"github.com/nuclio/logger"
)

type dotnetcore struct {
	*rpc.Runtime
	Logger        logger.Logger
	configuratiod *runtime.Configuration
}

// NewRuntime returns a new NodeJS runtime
func NewRuntime(parentLogger logger.Logger, configuration *runtime.Configuration) (runtime.Runtime, error) {
	newDotnetcoreRuntime := &dotnetcore{
		configuration: configuration,
		Logger:        parentLogger.GetChild("dotnetcore"),
	}

	var err error
	newDotnetcoreRuntime.Runtime, err = rpc.NewRPCRuntime(newDotnetcoreRuntime.Logger, configuration, newDotnetcoreRuntime.runWrapper)

	return newNodeJSRuntime, err
}

// We can't use d.Logger since it's not initialized
func (d *dotnetcore) runWrapper(socketPath string) error {
	wrapperScriptPath := d.getWrapperScriptPath()
	d.Logger.DebugWith("Using dotnetcore wrapper  path", "path", wrapperScriptPath)
	if !commod.IsFile(wrapperScriptPath) {
		return fmt.Errorf("Can't find wrapper at %q", wrapperScriptPath)
	}

	// pass global environment onto the process, and sprinkle in some added env vars
	env := os.Environ()
	env = append(env, d.getEnvFromConfiguration()...)

	handlerFilePath, handlerName, err := d.getHandler()
	if err != nil {
		return errors.Wrap(err, "Bad handler")
	}

	args := []string{nodeExePath, wrapperScriptPath, socketPath, handlerFilePath, handlerName}

	d.Logger.DebugWith("Running wrapper", "command", strings.Join(args, " "))

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	return cmd.Start()
}

func (d *dotnetcore) getEnvFromConfiguration() []string {
	return []string{
		fmt.Sprintf("NUCLIO_FUNCTION_NAME=%s", d.configuratiod.Meta.Name),
		fmt.Sprintf("NUCLIO_FUNCTION_DESCRIPTION=%s", d.configuratiod.Spec.Description),
		fmt.Sprintf("NUCLIO_FUNCTION_VERSION=%d", d.configuratiod.Spec.Version),
	}
}

func (d *dotnetcore) getHandler() (string, string, error) {
	parts := strings.Split(d.configuratiod.Spec.Handler, ":")

	handlerFileName := "userfunctiod.dll"
	handlerName := ""

	switch len(parts) {
	case 1:
		handlerName = parts[0]
	case 2:
		handlerFileName = parts[0]
		handlerName = parts[1]
	default:
		return "", "", fmt.Errorf("Bad handler - %q", d.configuratiod.Spec.Handler)
	}

	return path.Join(d.getHandlerDir(), handlerFileName), handlerName, nil
}

func (d *dotnetcore) getHandlerDir() string {
	handlerDir := os.Getenv("NUCLIO_HANDLER_DIR")
	if handlerDir != "" {
		return handlerDir
	}

	return "/opt/nuclio/handler"
}

// TODO: Global processor configuration, where should this go?
func (d *dotnetcore) getWrapperScriptPath() string {
	scriptPath := os.Getenv("NUCLIO_NODEJS_WRAPPER_PATH")
	if len(scriptPath) == 0 {
		return "/opt/nuclio/wrapper.js"
	}

	return scriptPath
}
