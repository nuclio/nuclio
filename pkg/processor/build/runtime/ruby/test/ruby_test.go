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
