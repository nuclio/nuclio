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

package shell

import (
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/stretchr/testify/suite"
)

// nuclio.TriggerInfoProvider interface
type TestTriggerInfoProvider struct{}

func (ti *TestTriggerInfoProvider) GetClass() string { return "test class" }
func (ti *TestTriggerInfoProvider) GetKind() string  { return "test kind" }
func (ti *TestTriggerInfoProvider) GetName() string  { return "test name" }

type ShellRuntimeSuite struct {
	processorsuite.TestSuite
	runtimeInstance runtime.Runtime

	tempRuntimeHandlerDir string
}

func (suite *ShellRuntimeSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()
	suite.Runtime = "shell"
	configuration, err := NewConfiguration(suite.resolveRuntimeConfiguration(suite.Logger))
	suite.Require().NoError(err, "Failed to create new configuration")

	suite.tempRuntimeHandlerDir = os.Getenv("NUCLIO_SHELL_HANDLER_DIR")
	err = os.Setenv("NUCLIO_SHELL_HANDLER_DIR", path.Join(suite.GetTestFunctionsDir(),
		suite.Runtime, "timeout"))
	suite.Require().NoError(err, "Failed to set NUCLIO_SHELL_HANDLER_DIR env")

	configuration.Spec.Handler = "timeout.sh:main"

	suite.runtimeInstance, err = NewRuntime(suite.Logger, configuration)
	suite.Require().NoError(err, "Failed to create new shell runtime")
}

func (suite *ShellRuntimeSuite) TearDownSuite() {
	suite.Require().NoError(os.Setenv("NUCLIO_SHELL_HANDLER_DIR", suite.tempRuntimeHandlerDir))
}

func (suite *ShellRuntimeSuite) TestExecute() {
	eventInstance := &nuclio.MemoryEvent{
		Body: []byte("sleep 0.1"),
	}
	eventInstance.SetTriggerInfoProvider(&TestTriggerInfoProvider{})
	response, err := suite.runtimeInstance.ProcessEvent(eventInstance, suite.Logger)
	suite.Require().NotNil(response)
	suite.Require().NoError(err)

	nuclioResponse := response.(nuclio.Response)
	suite.Require().Equal(http.StatusOK, nuclioResponse.StatusCode)

}

func (suite *ShellRuntimeSuite) TestTimeout() {

	// restart runtime after waiting a bit
	go func() {
		time.Sleep(200 * time.Millisecond)

		// restart runtime
		err := suite.runtimeInstance.Restart()
		suite.Require().NoError(err)
	}()

	// compile event
	eventInstance := &nuclio.MemoryEvent{}
	eventInstance.SetTriggerInfoProvider(&TestTriggerInfoProvider{})

	// process event
	_, err := suite.runtimeInstance.ProcessEvent(eventInstance, suite.Logger)
	suite.Require().Error(err)

	// error should be with status, to inform user his request has timed out
	responseError := err.(*nuclio.ErrorWithStatusCode)
	suite.Require().Equal(http.StatusRequestTimeout, responseError.StatusCode())
}

func (suite *ShellRuntimeSuite) resolveRuntimeConfiguration(loggerInstance logger.Logger) *runtime.Configuration {
	return &runtime.Configuration{
		FunctionLogger: loggerInstance,
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Meta: functionconfig.Meta{},
				Spec: functionconfig.Spec{},
			},
			PlatformConfig: &platformconfig.Config{},
		},
	}
}

func TestShellRuntimeSuite(t *testing.T) {
	suite.Run(t, new(ShellRuntimeSuite))
}
