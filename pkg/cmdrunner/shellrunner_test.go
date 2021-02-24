// +build test_unit

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

package cmdrunner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nuclio/nuclio/pkg/loggerus"

	"github.com/nuclio/logger"
	"github.com/stretchr/testify/suite"
)

type ShellRunnerTestSuite struct {
	suite.Suite
	logger      logger.Logger
	shellRunner ShellRunner
	runOptions  *RunOptions
}

func (suite *ShellRunnerTestSuite) SetupTest() {

	suite.logger, _ = loggerus.CreateTestLogger("test")
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
	cmd := exec.Command(suite.shellRunner.shell, "-c", `echo "foo1 foo2" ; echo "foo3">&2`)
	suite.runOptions.CaptureOutputMode = CaptureOutputModeCombined

	var runResult RunResult
	err := suite.shellRunner.runAndCaptureOutput(cmd, suite.runOptions, &runResult)
	suite.Require().NoError(err, "Failed to run command")

	suite.Require().Equal("foo1 foo2\nfoo3\n", runResult.Output)
	suite.Require().Empty(runResult.Stderr)
}

func (suite *ShellRunnerTestSuite) TestRunAndCaptureOutputStdoutReturnsStdoutAndStderr() {
	cmd := exec.Command(suite.shellRunner.shell, "-c", `echo "foo1 foo2" ; echo "foo3">&2`)
	suite.runOptions.CaptureOutputMode = CaptureOutputModeStdout

	var runResult RunResult
	err := suite.shellRunner.runAndCaptureOutput(cmd, suite.runOptions, &runResult)
	suite.Require().NoError(err, "Failed to run command")

	suite.Require().Equal("foo1 foo2\n", runResult.Output)
	suite.Require().Equal("foo3\n", runResult.Stderr)
}

func (suite *ShellRunnerTestSuite) TestRunAndCaptureOutputCombinedRedactsStrings() {
	cmd := exec.Command(suite.shellRunner.shell, "-c", `echo "foo1 foo2 secret" ; echo "foo3password">&2`)
	suite.runOptions.CaptureOutputMode = CaptureOutputModeCombined
	suite.runOptions.LogRedactions = []string{"password", "secret"}

	var runResult RunResult
	err := suite.shellRunner.runAndCaptureOutput(cmd, suite.runOptions, &runResult)
	suite.Require().NoError(err, "Failed to run command")

	suite.Require().Equal("foo1 foo2 [redacted]\nfoo3[redacted]\n", runResult.Output)
	suite.Require().Empty(runResult.Stderr)
}

func (suite *ShellRunnerTestSuite) TestRunAndCaptureOutputCombinedRedactsStringsFromStdoutAndStderr() {
	cmd := exec.Command(suite.shellRunner.shell, "-c", `echo "foo1 foo2 secret" ; echo "foo3password">&2`)
	suite.runOptions.CaptureOutputMode = CaptureOutputModeStdout
	suite.runOptions.LogRedactions = []string{"password", "secret"}

	var runResult RunResult
	err := suite.shellRunner.runAndCaptureOutput(cmd, suite.runOptions, &runResult)
	suite.Require().NoError(err, "Failed to run command")

	suite.Require().Equal("foo1 foo2 [redacted]\n", runResult.Output)
	suite.Require().Equal("foo3[redacted]\n", runResult.Stderr)
}

func TestShellRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(ShellRunnerTestSuite))
}
