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
	"testing"

	"github.com/nuclio/nuclio/pkg/processor/build/runtime/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	buildsuite.TestSuite
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()

	suite.TestSuite.RuntimeSuite = suite
	suite.TestSuite.ArchivePattern = "ruby"
}

func (suite *TestSuite) GetFunctionInfo(functionName string) buildsuite.FunctionInfo {
	functionInfo := buildsuite.FunctionInfo{
		Runtime: "ruby",
	}

	switch functionName {

	case "json-parser-with-function-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "json-parser-with-function-config", "ruby"}

	case "json-parser-with-inline-function-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "json-parser-with-inline-function-config", "ruby", "parser.rb"}
		functionInfo.Handler = "parser:main"

	case "invalid-inline-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "invalid-inline-config", "ruby", "parser.rb"}
		functionInfo.Handler = "parser:main"

	case "reverser":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "reverser", "ruby", "reverser.rb"}
		functionInfo.Handler = "reverser:main"

	case "long-initialization":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "long-initialization", "ruby", "sleepy.rb"}
		functionInfo.Handler = "sleepy:main"

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
