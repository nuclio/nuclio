//go:build test_integration

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

package shell

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

// nuclio.TriggerInfoProvider interface
type TestTriggerInfoProvider struct{}

func (ti *TestTriggerInfoProvider) GetClass() string { return "test class" }
func (ti *TestTriggerInfoProvider) GetKind() string  { return "test kind" }
func (ti *TestTriggerInfoProvider) GetName() string  { return "test name" }

// testEventWithHeaders extends MemoryEvent with working GetHeaderByteSlice and
// GetHeaderString implementations. MemoryEvent.GetHeader reads from the Headers
// map, but the AbstractEvent base's GetHeaderByteSlice always returns empty,
// so GetHeaderString (which calls GetHeaderByteSlice) never sees the map values.
type testEventWithHeaders struct {
	nuclio.MemoryEvent
}

func (event *testEventWithHeaders) GetHeaderByteSlice(key string) []byte {
	header := event.GetHeader(key)
	if header == nil {
		return nil
	}
	switch typedValue := header.(type) {
	case string:
		return []byte(typedValue)
	case []byte:
		return typedValue
	default:
		return nil
	}
}

func (event *testEventWithHeaders) GetHeaderString(key string) string {
	return string(event.GetHeaderByteSlice(key))
}

type ShellRuntimeSuite struct {
	suite.Suite

	runtimeInstance       runtime.Runtime
	logger                logger.Logger
	runtime               string
	tempRuntimeHandlerDir string
}

func (suite *ShellRuntimeSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.runtime = "shell"
	configuration, err := NewConfiguration(suite.resolveRuntimeConfiguration(suite.logger))
	suite.Require().NoError(err, "Failed to create new configuration")

	suite.tempRuntimeHandlerDir = os.Getenv("NUCLIO_SHELL_HANDLER_DIR")
	err = os.Setenv("NUCLIO_SHELL_HANDLER_DIR", path.Join(
		common.GetSourceDir(),
		"test",
		"_functions",
		suite.runtime,
		"timeout"))
	suite.Require().NoError(err, "Failed to set NUCLIO_SHELL_HANDLER_DIR env")

	configuration.Spec.Handler = "timeout.sh:main"

	suite.runtimeInstance, err = NewRuntime(suite.logger, configuration)
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
	response, err := suite.runtimeInstance.ProcessEvent(eventInstance, suite.logger)
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
	_, err := suite.runtimeInstance.ProcessEvent(eventInstance, suite.logger)
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
	if testing.Short() {
		return
	}
	suite.Run(t, new(ShellRuntimeSuite))
}

// CommandInjectionTestSuite demonstrates CVE-2026-29042 (OS Command Injection) in the shell
// runtime's X-Nuclio-Arguments header processing. When the handler resolves to a
// command in PATH, the runtime uses `sh -c` to execute it,  joining
// the command and user-supplied arguments into a single string without sanitization
// This allows shell metacharacters in the header value to break
// out of the intended command and execute arbitrary commands with the container's
// privileges.
type CommandInjectionTestSuite struct {
	suite.Suite
	runtimeInstance       runtime.Runtime
	logger                logger.Logger
	tempRuntimeHandlerDir string
}

func (suite *CommandInjectionTestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	configuration, err := NewConfiguration(suite.resolveRuntimeConfiguration())
	suite.Require().NoError(err, "Failed to create configuration")

	// Point handler dir to a directory where 'true' does not exist as a file,
	// so the runtime resolves it via PATH and sets commandInPath=true.
	// 'true' is ideal because it produces no output and ignores all arguments,
	// so any injected command output (e.g. "INJECTED") can only appear if
	// shell metacharacters were actually interpreted.
	suite.tempRuntimeHandlerDir = os.Getenv("NUCLIO_SHELL_HANDLER_DIR")
	err = os.Setenv("NUCLIO_SHELL_HANDLER_DIR", os.TempDir())
	suite.Require().NoError(err, "Failed to set NUCLIO_SHELL_HANDLER_DIR")

	configuration.Spec.Handler = "true:main"

	suite.runtimeInstance, err = NewRuntime(suite.logger, configuration)
	suite.Require().NoError(err, "Failed to create shell runtime")
}

func (suite *CommandInjectionTestSuite) TearDownSuite() {
	suite.Require().NoError(os.Setenv("NUCLIO_SHELL_HANDLER_DIR", suite.tempRuntimeHandlerDir))
}

func (suite *CommandInjectionTestSuite) TestShellMetacharactersAreNotInterpreted() {
	testCases := []struct {
		name                string
		argumentsPayload    string
		forbiddenInResponse string
	}{
		{
			name:                "semicolon must not break out of command",
			argumentsPayload:    "; echo INJECTED ;",
			forbiddenInResponse: "INJECTED",
		},
		{
			name:                "backticks must not perform command substitution",
			argumentsPayload:    "`echo INJECTED`",
			forbiddenInResponse: "INJECTED",
		},
		{
			name:                "dollar-paren must not perform command substitution",
			argumentsPayload:    "$(echo INJECTED)",
			forbiddenInResponse: "INJECTED",
		},
		{
			name:                "pipe must not redirect to another command",
			argumentsPayload:    "| echo INJECTED",
			forbiddenInResponse: "INJECTED",
		},
		{
			name:                "double-ampersand must not chain commands",
			argumentsPayload:    "&& echo INJECTED",
			forbiddenInResponse: "INJECTED",
		},
		{
			name:                "semicolon must not allow reading sensitive files",
			argumentsPayload:    "; cat /etc/passwd ;",
			forbiddenInResponse: "root:",
		},
	}

	for _, testCase := range testCases {
		suite.Run(testCase.name, func() {
			responseBody := suite.getResponseBodyForArguments(testCase.argumentsPayload)
			suite.Require().NotContains(responseBody, testCase.forbiddenInResponse)
		})
	}
}

func (suite *CommandInjectionTestSuite) TestServiceAccountTokenCannotBeExfiltratedViaInjection() {

	// Simulates the attack described in the vulnerability report: reading a
	// Kubernetes ServiceAccount token from the filesystem via command injection.
	// A temp file stands in for /var/run/secrets/kubernetes.io/serviceaccount/token.
	fakeTokenContent := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.fake-sa-token"
	tempFile, err := os.CreateTemp("", "fake-sa-token-*")
	suite.Require().NoError(err)
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(fakeTokenContent)
	suite.Require().NoError(err)
	suite.Require().NoError(tempFile.Close())

	responseBody := suite.getResponseBodyForArguments(fmt.Sprintf("; cat %s ;", tempFile.Name()))
	suite.Require().NotContains(responseBody, fakeTokenContent,
		"Command injection must not allow reading ServiceAccount tokens from the filesystem")
}

// getResponseBodyForArguments sends an event with the given X-Nuclio-Arguments header
// and returns the response body as a string. Returns empty string if the runtime
// returns an error or nil response, since either outcome means no injection occurred.
func (suite *CommandInjectionTestSuite) getResponseBodyForArguments(arguments string) string {
	event := &testEventWithHeaders{
		MemoryEvent: nuclio.MemoryEvent{
			Body: []byte("test"),
			Headers: map[string]interface{}{
				headers.Arguments: arguments,
			},
		},
	}
	event.SetTriggerInfoProvider(&TestTriggerInfoProvider{})

	response, err := suite.runtimeInstance.ProcessEvent(event, suite.logger)
	if err != nil || response == nil {
		return ""
	}

	return string(response.(nuclio.Response).Body)
}

func (suite *CommandInjectionTestSuite) resolveRuntimeConfiguration() *runtime.Configuration {
	return &runtime.Configuration{
		FunctionLogger: suite.logger,
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Meta: functionconfig.Meta{},
				Spec: functionconfig.Spec{},
			},
			PlatformConfig: &platformconfig.Config{},
		},
	}
}

func TestCommandInjectionTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(CommandInjectionTestSuite))
}
