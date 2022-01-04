//go:build test_unit

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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type ShellRunnerTestSuite struct {
	suite.Suite
	logger      logger.Logger
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

func (suite *ShellRunnerTestSuite) TestStream() {
	for _, testCase := range []struct {
		name          string
		streamCommand string
	}{
		{
			name:          "Stream",
			streamCommand: "echo something",
		},
		{
			name: "StreamCancelled",
			streamCommand: func() string {

				// stream the entire file contents and then follow
				return fmt.Sprintf(`tail -n +1 -f %s`, path.Join(common.GetSourceDir(), "README.md"))
			}(),
		},
	} {
		suite.Run(testCase.name, func() {
			ctx, cancel := context.WithCancel(context.Background())
			fileReader, err := suite.shellRunner.Stream(ctx,
				&RunOptions{},
				testCase.streamCommand,
			)
			suite.Require().NoError(err)

			buffer := bytes.NewBuffer([]byte{})
			bufferIsFilled := make(chan bool)
			go func() {
				for buffer.Len() == 0 {
					suite.logger.DebugWithCtx(ctx, "Filling buffer with commands output")
					io.Copy(buffer, fileReader) // nolint: errcheck
					time.Sleep(250 * time.Millisecond)
				}
				bufferIsFilled <- true

				// let it stream for a second and then stop it
				suite.logger.DebugWithCtx(ctx, "Got some data, cancelling context")
				cancel()
			}()

			// In case stream channel is still open
			time.AfterFunc(3*time.Second, func() {
				suite.logger.DebugWithCtx(ctx, "Forcefully cancelling context")
				cancel()
			})

			// read all streamed data
			suite.logger.DebugWithCtx(ctx, "Waiting for buffer to get filled")
			<-bufferIsFilled

			// sanity
			suite.Require().NotEmpty(buffer.String())

			// wait for context termination
			<-ctx.Done()
			suite.logger.DebugWithCtx(ctx, "Context is terminated")

			// let the process wrap up and close its FDs
			time.Sleep(1 * time.Second)

			// `stream` is running with a context, once it is being terminated (or cancelled), the reader should be closed
			// this ensure it has been closed.
			err = fileReader.Close()
			suite.Require().Error(err)
			suite.Require().Contains(err.Error(), "already closed", "should have been closed")

		})
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
