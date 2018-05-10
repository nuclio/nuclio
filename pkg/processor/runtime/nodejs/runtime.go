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

package nodejs

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc"

	"github.com/nuclio/logger"
)

type nodejs struct {
	*rpc.Runtime
	Logger        logger.Logger
	configuration *runtime.Configuration
}

// NewRuntime returns a new NodeJS runtime
func NewRuntime(parentLogger logger.Logger, configuration *runtime.Configuration) (runtime.Runtime, error) {
	newNodeJSRuntime := &nodejs{
		configuration: configuration,
		Logger:        parentLogger.GetChild("nodejs"),
	}

	var err error
	newNodeJSRuntime.Runtime, err = rpc.NewRPCRuntime(newNodeJSRuntime.Logger, configuration, newNodeJSRuntime.runWrapper, rpc.UnixSocket)

	return newNodeJSRuntime, err
}

// We can't use n.Logger since it's not initialized
func (n *nodejs) runWrapper(socketPath string) (*os.Process, error) {
	wrapperScriptPath := n.getWrapperScriptPath()
	n.Logger.DebugWith("Using nodejs wrapper script path", "path", wrapperScriptPath)
	if !common.IsFile(wrapperScriptPath) {
		return nil, fmt.Errorf("Can't find wrapper at %q", wrapperScriptPath)
	}

	nodeExePath, err := n.getNodeExePath()
	if err != nil {
		n.Logger.ErrorWith("Can't find node exe", "error", err)
		return nil, errors.Wrap(err, "Can't find node exe")
	}
	n.Logger.DebugWith("Using node executable", "path", nodeExePath)

	// pass global environment onto the process, and sprinkle in some added env vars
	env := os.Environ()
	env = append(env, n.getEnvFromConfiguration()...)

	handlerFilePath, handlerName, err := n.getHandler()
	if err != nil {
		return nil, errors.Wrap(err, "Bad handler")
	}

	args := []string{nodeExePath, wrapperScriptPath, socketPath, handlerFilePath, handlerName}

	n.Logger.DebugWith("Running wrapper", "command", strings.Join(args, " "))

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	return cmd.Process, cmd.Start()
}

func (n *nodejs) getEnvFromConfiguration() []string {
	return []string{
		fmt.Sprintf("NUCLIO_FUNCTION_NAME=%s", n.configuration.Meta.Name),
		fmt.Sprintf("NUCLIO_FUNCTION_DESCRIPTION=%s", n.configuration.Spec.Description),
		fmt.Sprintf("NUCLIO_FUNCTION_VERSION=%d", n.configuration.Spec.Version),
	}
}

func (n *nodejs) getHandler() (string, string, error) {
	// TODO: support file names, use functionconfig.ParseHandler

	parts := strings.Split(n.configuration.Spec.Handler, ":")

	handlerFileName := "handler.js"
	handlerName := ""

	switch len(parts) {
	case 1:
		handlerName = parts[0]
	case 2:
		handlerFileName = parts[0]
		handlerName = parts[1]
	default:
		return "", "", fmt.Errorf("Bad handler - %q", n.configuration.Spec.Handler)
	}

	return path.Join(n.getHandlerDir(), handlerFileName), handlerName, nil
}

func (n *nodejs) getHandlerDir() string {
	handlerDir := os.Getenv("NUCLIO_HANDLER_DIR")
	if handlerDir != "" {
		return handlerDir
	}

	return "/opt/nuclio"
}

// TODO: Global processor configuration, where should this go?
func (n *nodejs) getWrapperScriptPath() string {
	scriptPath := os.Getenv("NUCLIO_NODEJS_WRAPPER_PATH")
	if len(scriptPath) == 0 {
		return "/opt/nuclio/wrapper.js"
	}

	return scriptPath
}

func (n *nodejs) getNodeExePath() (string, error) {
	exePath := os.Getenv("NUCLIO_NODEJS_EXE")
	if exePath != "" {
		return exePath, nil
	}
	baseName := "node"

	return exec.LookPath(baseName)
}
