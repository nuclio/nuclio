package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/util/cmdrunner"
	nucliozap "github.com/nuclio/nuclio/pkg/zap"
)

type CmdLineSuite struct {
	suite.Suite

	logger     nuclio.Logger
	imageName  string
	cmd        *cmdrunner.CmdRunner
	runOptions *cmdrunner.RunOptions
}

func (suite *CmdLineSuite) failOnError(err error, fmt string, args ...interface{}) {
	if err != nil {
		suite.FailNowf(err.Error(), fmt, args...)
	}
}

func (suite *CmdLineSuite) gitRoot() string {
	out, err := suite.cmd.Run(nil, "git rev-parse --show-toplevel")
	suite.failOnError(err, "Can't find git root")
	return strings.TrimSpace(out)
}

func (suite *CmdLineSuite) SetupSuite() {
	var loggerLevel nucliozap.Level
	if testing.Verbose() {
		loggerLevel = nucliozap.DebugLevel
	} else {
		loggerLevel = nucliozap.InfoLevel
	}

	zap, err := nucliozap.NewNuclioZap("test-build", loggerLevel)
	suite.failOnError(err, "Can't create logger")
	suite.logger = zap
	suite.cmd, err = cmdrunner.NewCmdRunner(suite.logger)
	suite.failOnError(err, "Can't create runner")
	root := suite.gitRoot()
	suite.runOptions = &cmdrunner.RunOptions{WorkingDir: &root}

	suite.imageName = "bld-test"
	suite.logger.InfoWith("Image name", "name", suite.imageName)
}

func (suite *CmdLineSuite) TearDownSuite() {
	suite.cmd.Run(nil, "docker rmi %s", suite.imageName)
}

func (suite *CmdLineSuite) TestBuild() {
	_, err := suite.cmd.Run(suite.runOptions, "go build ./cmd/nuclio-build")
	suite.failOnError(err, "Can't build nuclio-build")
	_, err = suite.cmd.Run(suite.runOptions, "./nuclio-build --verbose -n %s ../nuclio-sdk/examples/hello-world", suite.imageName)
	suite.failOnError(err, "Can't run nuclio-build")
}

func TestCmdLine(t *testing.T) {
	suite.Run(t, new(CmdLineSuite))
}
