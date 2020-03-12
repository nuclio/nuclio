package test

import (
	"os"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/python"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type testRunTimeSuite struct {
	suite.Suite
	Logger         logger.Logger
	Version        version.Info
	FunctionConfig *functionconfig.Config
}

func (suite *testRunTimeSuite) SetupSuite() {
	var err error
	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Failed to create nuclio zap logger")
	suite.Version = version.Info{}
	err = os.Setenv("NUCLIO_PYTHON_EXE_PATH", "pythonX")
	suite.Require().NoError(err, "Failed to set nuclio python exe path")
	suite.FunctionConfig = functionconfig.NewConfig()
}

func (suite *testRunTimeSuite) TearDownTestSuite() {
	var err error
	err = os.Unsetenv("NUCLIO_PYTHON_EXE_PATH")
	suite.Require().NoError(err, "Failed to unset nuclio python exe path")
}

func (suite *testRunTimeSuite) TestPythonExePath() {
	pythonRuntime := suite.buildRuntime()
	processorDockerInfo, err := pythonRuntime.GetProcessorDockerfileInfo(&suite.Version, "a")
	suite.Require().NoError(err)
	suite.True(strings.HasPrefix(processorDockerInfo.Directives["postCopy"][0].Value, "pythonX"),
		"Python exe path should start with test overridden value")
}

func (suite *testRunTimeSuite) buildRuntime() *python.Python {
	pyRuntime, err := runtime.NewAbstractRuntime(suite.Logger, "/tmp/stagingDir", suite.FunctionConfig)
	suite.Require().NoError(err)
	return &python.Python{
		AbstractRuntime: pyRuntime,
	}
}

func TestRuntime(t *testing.T) {
	suite.Run(t, new(testRunTimeSuite))
}
