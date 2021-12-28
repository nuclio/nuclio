//go:build test_integration && test_local

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
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/test/suite"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	buildsuite.TestSuite
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()
	suite.TestSuite.Runtime = "java"

	suite.TestSuite.RuntimeSuite = suite
	suite.TestSuite.ArchivePattern = "java"
}

func (suite *TestSuite) GetFunctionInfo(functionName string) buildsuite.FunctionInfo {
	functionInfo := buildsuite.FunctionInfo{
		Runtime: suite.TestSuite.Runtime,
	}

	switch functionName {

	case "reverser":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "reverser", "java", "Reverser.java"}
		functionInfo.Handler = "Reverser"

	case "json-parser-with-function-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "json-parser-with-function-config", "java"}
		functionInfo.Handler = "JsonParser"

	case "json-parser-with-inline-function-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "json-parser-with-inline-function-config", "java", "JsonParser.java"}
		functionInfo.Handler = "JsonParser"

	case "invalid-inline-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "invalid-inline-config", "java", "JsonParser.java"}
		functionInfo.Handler = "JsonParser"

	default:
		suite.Logger.InfoWith("Test skipped", "functionName", functionName)
		functionInfo.Skip = true
	}

	return functionInfo
}

func (suite *TestSuite) TestBuildWithCustomGradleScript() {
	createFunctionOptions := suite.GetDeployOptions("custom-gradle-script",
		suite.GetFunctionPath(suite.GetTestFunctionsDir(), "java", "custom-gradle-script"))

	createFunctionOptions.FunctionConfig.Spec.Handler = "JsonParser"
	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestBody:          `{"a": 100, "return_this": "returned value"}`,
			ExpectedResponseBody: "returned value",
		})
}

func (suite *TestSuite) TestBuildWithCustomRepositories() {
	createFunctionOptions := suite.GetDeployOptions("reverser",
		suite.GetFunctionPath(suite.GetTestFunctionsDir(), "common", "reverser", "java", "Reverser.java"))

	createFunctionOptions.FunctionConfig.Spec.Handler = "Reverser"
	createFunctionOptions.FunctionConfig.Spec.Build.RuntimeAttributes = map[string]interface{}{
		"repositories": []string{"mavenCentral()", "jcenter()"},
	}

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestBody:          "abcd",
			ExpectedResponseBody: "dcba",
		})
}

func (suite *TestSuite) TestBuildWithJar() {
	createFunctionOptions := suite.GetDeployOptions("reverser",
		suite.GetFunctionPath(suite.GetTestFunctionsDir(), "java", "reverser-jar", "reverser.jar"))

	createFunctionOptions.FunctionConfig.Spec.Handler = "Reverser"
	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestBody:          "abcd",
			ExpectedResponseBody: "dcba",
		})
}

func (suite *TestSuite) TestBuildWithJarFromURL() {
	createFunctionOptions := suite.GetDeployOptions("reverser",
		suite.GetFunctionPath(suite.GetTestFunctionsDir(), "java", "reverser-jar", "reverser.jar"))

	createFunctionOptions.FunctionConfig.Spec.Handler = "Reverser"
	suite.DeployFunctionFromURL(createFunctionOptions, &httpsuite.Request{
		RequestBody:          "abcd",
		ExpectedResponseBody: "dcba",
	})
}

func (suite *TestSuite) getDeployOptions(functionName string) *platform.CreateFunctionOptions {
	functionInfo := suite.RuntimeSuite.GetFunctionInfo(functionName)

	if functionInfo.Skip {
		suite.T().Skip()
	}

	createFunctionOptions := suite.GetDeployOptions(functionName,
		path.Join(functionInfo.Path...))

	createFunctionOptions.FunctionConfig.Spec.Handler = functionInfo.Handler
	createFunctionOptions.FunctionConfig.Spec.Runtime = functionInfo.Runtime

	return createFunctionOptions
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
