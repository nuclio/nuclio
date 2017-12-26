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

package test

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/processor/build/runtime/test/suite"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	buildsuite.TestSuite
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()

	suite.TestSuite.RuntimeSuite = suite
}

func (suite *TestSuite) TestBuildBinaryWithStdin() {
	deployOptions := suite.GetDeployOptions("reverser", "/dev/null")

	deployOptions.FunctionConfig.Spec.Runtime = "shell"
	deployOptions.FunctionConfig.Spec.Handler = "rev"

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildBinaryWithArguments() {
	deployOptions := suite.GetDeployOptions("echoer", "/dev/null")

	deployOptions.FunctionConfig.Spec.Runtime = "shell"
	deployOptions.FunctionConfig.Spec.Handler = "echo"
	deployOptions.FunctionConfig.Spec.RuntimeAttributes = map[string]interface{}{
		"arguments": "abcdef",
	}

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "GET",
			ExpectedResponseBody: "abcdef\n",
		})
}

func (suite *TestSuite) TestBuildBinaryWithArgumentsFromEvent() {
	deployOptions := suite.GetDeployOptions("echoer", "/dev/null")

	deployOptions.FunctionConfig.Spec.Runtime = "shell"
	deployOptions.FunctionConfig.Spec.Handler = "echo"
	deployOptions.FunctionConfig.Spec.RuntimeAttributes = map[string]interface{}{
		"arguments": "abcdef",
	}

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod: "GET",
			RequestHeaders: map[string]string{
				"x-nuclio-arguments": "123456",
			},
			ExpectedResponseBody: "123456\n",
		})
}

func (suite *TestSuite) TestBuildBinaryWithResponseHeaders() {
	deployOptions := suite.GetDeployOptions("echoer", "/dev/null")
	expectedResponseHeaders := map[string]string{
		"header1": "value1",
		"header2": "value2",
	}

	deployOptions.FunctionConfig.Spec.Runtime = "shell"
	deployOptions.FunctionConfig.Spec.Handler = "echo"
	deployOptions.FunctionConfig.Spec.RuntimeAttributes = map[string]interface{}{
		"arguments": "abcdef",
		"responseHeaders": map[string]string{
			"header1": "value1",
			"header2": "value2",
		},
	}

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod: "GET",
			RequestHeaders: map[string]string{
				"x-nuclio-arguments": "123456",
			},
			ExpectedResponseBody:    "123456\n",
			ExpectedResponseHeaders: expectedResponseHeaders,
		})
}

// TODO: Fix TestBuildBinaryWithResponseHeadersFailsOnInvalidResponseHeadersType after failed container detection is implemented
/*
func (suite *TestSuite) TestBuildBinaryWithResponseHeadersFailsOnInvalidResponseHeadersType() {
	deployOptions := suite.GetDeployOptions("echoer", "/dev/null")

	deployOptions.FunctionConfig.Spec.Runtime = "shell"
	deployOptions.FunctionConfig.Spec.Handler = "echo"
	deployOptions.FunctionConfig.Spec.RuntimeAttributes = map[string]interface{}{
		"arguments":       "abcdef",
		"responseHeaders": "\"header1\": \"value1\", \"header2\": \"value2\"",
	}

	expectedStatusCode := 500

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod: "GET",
			RequestHeaders: map[string]string{
				"x-nuclio-arguments": "123456",
			},
			ExpectedResponseStatusCode: &expectedStatusCode,
			ExpectedResponseBody:       nil,
		})
}
*/

func (suite *TestSuite) GetFunctionInfo(functionName string) buildsuite.FunctionInfo {
	functionInfo := buildsuite.FunctionInfo{
		Runtime: "shell",
	}

	switch functionName {

	case "reverser":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "reverser", "shell", "reverser.sh"}
		functionInfo.Handler = "reverser.sh:main"

	case "json-parser-with-function-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "json-parser-with-function-config", "shell"}

	case "json-parser-with-inline-function-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "json-parser-with-inline-function-config", "shell", "parser.sh"}

	default:
		suite.Logger.InfoWith("Test skipped", "functionName", functionName)

		functionInfo.Skip = true
	}

	return functionInfo
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
