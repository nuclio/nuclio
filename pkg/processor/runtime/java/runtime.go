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

package java

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/connection"
	"github.com/nuclio/nuclio/pkg/processor/runtime/rpc/encoder"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type java struct {
	*rpc.AbstractRuntime
	Logger        logger.Logger
	configuration *runtime.Configuration
}

// NewRuntime returns a new Java runtime
func NewRuntime(parentLogger logger.Logger, configuration *runtime.Configuration) (runtime.Runtime, error) {
	var err error

	newJavaRuntime := &java{
		configuration: configuration,
		Logger:        parentLogger.GetChild("logger"),
	}

	newJavaRuntime.AbstractRuntime, err = rpc.NewAbstractRuntime(newJavaRuntime.Logger,
		configuration,
		newJavaRuntime)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create runtime")
	}

	return newJavaRuntime, nil
}

func (j *java) RunWrapper(ports []string, controlPort string) (*os.Process, error) {

	if len(ports) != 1 {
		return nil, fmt.Errorf("Java runtime doesn't support multiple ports processing")
	}

	jvmOptions, err := j.getJVMOptions()
	if err != nil {
		return nil, err
	}

	args := append([]string{"java"}, jvmOptions...)
	args = append(args, []string{
		"-jar", j.wrapperJarPath(),
		"-handler", j.handlerName(),
		"-port", ports[0],
		"-workerid", strconv.Itoa(j.configuration.WorkerID),
	}...)

	env := os.Environ()
	env = append(env, j.AbstractRuntime.GetEnvFromConfiguration()...)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	j.Logger.InfoWith("Running wrapper jar", "command", strings.Join(cmd.Args, " "))

	return cmd.Process, cmd.Start()
}

// GetSocketType returns the type of socket the runtime works with (unix/tcp)
func (j *java) GetSocketType() connection.SocketType {
	return connection.TCPSocket
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

func (j *java) getJVMOptions() ([]string, error) {
	attrs := j.configuration.Spec.RuntimeAttributes
	if attrs == nil {
		return nil, nil
	}

	rawJVMOptions, found := attrs["jvmOptions"]
	if !found {
		return nil, nil
	}

	jvmOptionsIface, ok := rawJVMOptions.([]interface{})
	if !ok {
		j.Logger.ErrorWith("jvmOptions is not a list", "options", rawJVMOptions)
		return nil, errors.Errorf("jvmOptions is not a list (%v : %T)", rawJVMOptions, rawJVMOptions)
	}

	jvmOptions := make([]string, len(jvmOptionsIface))
	for i, iface := range jvmOptionsIface {
		option, ok := iface.(string)
		if !ok {
			j.Logger.ErrorWith("Non-string JVM option", "index", i, "value", option)
			return nil, errors.Errorf("jvmOptions %d is not a string (%v : %T)", i, iface, iface)
		}
		jvmOptions[i] = option
	}

	return jvmOptions, nil
}

func (j *java) GetEventEncoder(writer io.Writer) encoder.EventEncoder {
	return encoder.NewEventJSONEncoder(j.Logger, writer)
}
