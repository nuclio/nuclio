//go:build test_unit

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

package rpc

import (
	"io"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type testRuntime struct {
	*AbstractRuntime
	wrapperProcess *os.Process
	wrapperConn    net.Conn
}

// NewRuntime returns a new Python runtime
func newTestRuntime(parentLogger logger.Logger, configuration *runtime.Configuration) (*testRuntime, error) {
	var err error

	newTestRuntime := &testRuntime{}

	newTestRuntime.AbstractRuntime, err = NewAbstractRuntime(parentLogger.GetChild("logger"),
		configuration,
		newTestRuntime)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create runtime")
	}

	return newTestRuntime, nil
}

func (r *testRuntime) RunWrapper(socketPath string) (*os.Process, error) {
	var err error
	cmd := exec.Command("sleep", "999999")
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	r.wrapperProcess = cmd.Process

	// Connect to runtime
	r.wrapperConn, err = net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}

	return cmd.Process, nil
}

func (r *testRuntime) GetEventEncoder(writer io.Writer) EventEncoder {
	return NewEventJSONEncoder(r.Logger, writer)
}

type RuntimeSuite struct {
	suite.Suite
	testRuntimeInstance *testRuntime
}

func (suite *RuntimeSuite) TestRestart() {
	var err error

	loggerInstance := suite.createLogger()
	configInstance := suite.createConfig(loggerInstance)

	suite.testRuntimeInstance, err = newTestRuntime(loggerInstance, configInstance)
	suite.Require().NoError(err, "Can't create runtime")

	err = suite.testRuntimeInstance.Start()
	suite.Require().NoError(err, "Can't start runtime")

	time.Sleep(1 * time.Second)

	oldPid := suite.testRuntimeInstance.wrapperProcess.Pid
	err = suite.testRuntimeInstance.Restart()
	suite.Require().NoError(err, "Can't restart runtime")
	suite.Require().NotEqual(oldPid, suite.testRuntimeInstance.wrapperProcess.Pid, "Wrapper process didn't change")
}

func (suite *RuntimeSuite) TearDownTest() {
	if suite.testRuntimeInstance != nil && suite.testRuntimeInstance.wrapperProcess != nil {
		suite.testRuntimeInstance.Stop() // nolint: errcheck
	}
}

func (suite *RuntimeSuite) createLogger() logger.Logger {
	loggerInstance, err := nucliozap.NewNuclioZapTest("rpc-runtime-test")
	suite.Require().NoError(err, "Can't create logger")

	return loggerInstance
}

func (suite *RuntimeSuite) createConfig(loggerInstance logger.Logger) *runtime.Configuration {
	return &runtime.Configuration{
		FunctionLogger: loggerInstance,
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Meta: functionconfig.Meta{
					Namespace: "test",
				},
				Spec: functionconfig.Spec{},
			},
			PlatformConfig: &platformconfig.Config{
				Kind: "docker",
			},
		},
	}
}

func TestRuntime(t *testing.T) {
	suite.Run(t, new(RuntimeSuite))
}
