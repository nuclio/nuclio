package cmdrunner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/suite"
)

type ShellRunnerTestSuite struct {
	suite.Suite
	logger      nuclio.Logger
	shellRunner ShellRunner
	runOptions  *RunOptions
}

func (suite *ShellRunnerTestSuite) SetupTest() {

	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	newShellRunner, err := NewShellRunner(suite.logger)
	if err != nil {
		panic("Failed to create command runner")
	}
	suite.shellRunner = *newShellRunner

	currentDirectory, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		suite.Fail("Failed to get current directory")
	}
	suite.runOptions = &RunOptions{
		WorkingDir: &currentDirectory,
	}
}

func (suite *ShellRunnerTestSuite) TestBadShell() {
	suite.shellRunner.SetShell("/bin/definitelynotashell")

	_, err := suite.shellRunner.Run(nil, `pwd`)
	suite.Require().Error(err)
}

func (suite *ShellRunnerTestSuite) TestRunAndCaptureOutputCombinedReturnsOutputAndNoStderr() {
	cmd := exec.Command(suite.shellRunner.shell, "-c", `echo "foo1 foo2\nfoo3"`)
	suite.runOptions.CaptureOutputMode = CaptureOutputModeCombined
	suite.runOptions.LogRedactions = []string{"password",}

	var runResult RunResult
	err := suite.shellRunner.runAndCaptureOutput(cmd, suite.runOptions, &runResult)
	suite.Require().NoError(err, "Failed to run command")

	suite.Require().Equal("foo1 foo2\nfoo3\n", runResult.Output)
	suite.Require().Empty(runResult.Stderr)
}

func (suite *ShellRunnerTestSuite) TestRunAndCaptureOutputStdoutReturnsStdoutAndStderr() {
	cmd := exec.Command(suite.shellRunner.shell, "-c", `echo "foo1 foo2" ; echo "foo3">&2`)
	suite.runOptions.CaptureOutputMode = CaptureOutputModeStdout
	suite.runOptions.LogRedactions = []string{"password",}

	var runResult RunResult
	err := suite.shellRunner.runAndCaptureOutput(cmd, suite.runOptions, &runResult)
	suite.Require().NoError(err, "Failed to run command")

	suite.Require().Equal("foo1 foo2\n", runResult.Output)
	suite.Require().Equal("foo3\n", runResult.Stderr)
}

func TestShellRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(ShellRunnerTestSuite))
}
