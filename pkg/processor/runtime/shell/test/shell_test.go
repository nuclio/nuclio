//go:build test_integration && test_local

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

package test

import (
	"fmt"
	"net/http"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/processor/runtime/shell"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
)

type TestSuite struct {
	httpsuite.TestSuite
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = "shell"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "shell", "test")
}

func (suite *TestSuite) TestOutputs() {
	statusOK := http.StatusOK
	statusInternalError := http.StatusInternalServerError

	expectedResponseHeaders := map[string]string{
		"content-type": "text/plain; charset=utf-8",
		"header1":      "value1",
	}

	createFunctionOptions := suite.GetDeployOptions("outputter",
		suite.GetFunctionPath("outputter"))

	createFunctionOptions.FunctionConfig.Spec.Handler = "outputter.sh:main"
	createFunctionOptions.FunctionConfig.Spec.Env = []v1.EnvVar{
		{Name: "ENV1", Value: "value1"},
		{Name: "ENV2", Value: "value2"},
	}
	createFunctionOptions.FunctionConfig.Spec.RuntimeAttributes = map[string]interface{}{
		"arguments":       "first second",
		"responseHeaders": map[string]interface{}{"header1": "value1"},
	}

	suite.DeployFunctionAndRequests(createFunctionOptions, []*httpsuite.Request{
		{
			Name:                       "return body",
			RequestBody:                "return_body",
			ExpectedResponseHeaders:    expectedResponseHeaders,
			ExpectedResponseBody:       "return_body\n",
			ExpectedResponseStatusCode: &statusOK,
		},
		{
			Name:                       "return environment variables",
			RequestBody:                "return_env",
			ExpectedResponseHeaders:    expectedResponseHeaders,
			ExpectedResponseBody:       "value1-value2\n",
			ExpectedResponseStatusCode: &statusOK,
		},
		{
			Name:                       "return error",
			RequestBody:                "return_error",
			ExpectedResponseStatusCode: &statusInternalError,
		},
		{
			Name:                       "return arguments",
			RequestBody:                "return_arguments",
			ExpectedResponseHeaders:    expectedResponseHeaders,
			ExpectedResponseBody:       "first-second\n",
			ExpectedResponseStatusCode: &statusOK,
		},
		{
			Name: "return overridden arguments",
			RequestHeaders: map[string]interface{}{
				headers.Arguments: "overridefirst overridesecond",
			},
			RequestBody:                "return_arguments",
			ExpectedResponseHeaders:    expectedResponseHeaders,
			ExpectedResponseBody:       "overridefirst-overridesecond\n",
			ExpectedResponseStatusCode: &statusOK,
		},
		{
			Name:        "return body on error",
			RequestBody: "return_error_with_message",
			ExpectedResponseBody: fmt.Sprintf(shell.ResponseErrorFormat,
				"exit status 1",
				"return_error_with_message\nsome_error\n"),
			ExpectedResponseStatusCode: &statusInternalError,
		},
	})
}

func (suite *TestSuite) TestStress() {

	// Create blastConfiguration using default configurations + changes for shell specification
	blastConfiguration := suite.NewBlastConfiguration()
	blastConfiguration.Handler = "outputter.sh:main"

	// Create stress test using suite.BlastHTTP
	suite.BlastHTTP(blastConfiguration)
}

// TestCommandInjectionIsBlocked verifies that shell metacharacters in the
// X-Nuclio-Arguments header are not interpreted when the handler is a PATH
// command (commandInPath=true). Uses 'true' which produces no output and
// ignores all arguments, so any injected command output can only appear if
// shell metacharacters were actually interpreted (CVE-2026-29042).
func (suite *TestSuite) TestCommandInjectionIsBlocked() {
	statusOK := http.StatusOK

	// Deploy 'true' as a PATH-based handler (no source code needed).
	createFunctionOptions := suite.GetDeployOptions("cmd-injection-test", "/dev/null")
	createFunctionOptions.FunctionConfig.Spec.Handler = "true"

	suite.DeployFunctionAndRequests(createFunctionOptions, []*httpsuite.Request{
		{
			Name:        "semicolon must not break out of command",
			RequestBody: "test",
			RequestHeaders: map[string]interface{}{
				headers.Arguments: "; echo INJECTED ;",
			},
			ExpectedResponseStatusCode: &statusOK,
			ExpectedResponseBody: func(body []byte) {
				suite.Require().NotContains(string(body), "INJECTED")
			},
		},
		{
			Name:        "backticks must not perform command substitution",
			RequestBody: "test",
			RequestHeaders: map[string]interface{}{
				headers.Arguments: "`echo INJECTED`",
			},
			ExpectedResponseStatusCode: &statusOK,
			ExpectedResponseBody: func(body []byte) {
				suite.Require().NotContains(string(body), "INJECTED")
			},
		},
		{
			Name:        "dollar-paren must not perform command substitution",
			RequestBody: "test",
			RequestHeaders: map[string]interface{}{
				headers.Arguments: "$(echo INJECTED)",
			},
			ExpectedResponseStatusCode: &statusOK,
			ExpectedResponseBody: func(body []byte) {
				suite.Require().NotContains(string(body), "INJECTED")
			},
		},
		{
			Name:        "pipe must not redirect to another command",
			RequestBody: "test",
			RequestHeaders: map[string]interface{}{
				headers.Arguments: "| echo INJECTED",
			},
			ExpectedResponseStatusCode: &statusOK,
			ExpectedResponseBody: func(body []byte) {
				suite.Require().NotContains(string(body), "INJECTED")
			},
		},
		{
			Name:        "double-ampersand must not chain commands",
			RequestBody: "test",
			RequestHeaders: map[string]interface{}{
				headers.Arguments: "&& echo INJECTED",
			},
			ExpectedResponseStatusCode: &statusOK,
			ExpectedResponseBody: func(body []byte) {
				suite.Require().NotContains(string(body), "INJECTED")
			},
		},
		{
			Name:        "semicolon must not allow reading sensitive files",
			RequestBody: "test",
			RequestHeaders: map[string]interface{}{
				headers.Arguments: "; cat /etc/passwd ;",
			},
			ExpectedResponseStatusCode: &statusOK,
			ExpectedResponseBody: func(body []byte) {
				suite.Require().NotContains(string(body), "root:")
			},
		},
	})
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
