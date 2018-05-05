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

package dockerclient

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type mockCmdRunner struct {
	mock.Mock
	expectedStdout   string
	expectedStderr   string
	expectedExitCode int
}

func newMockCmdRunner(expectedStdout, expectedStderr string, expectedErrorCode int) *mockCmdRunner {
	return &mockCmdRunner{
		expectedStdout:   expectedStdout,
		expectedStderr:   expectedStderr,
		expectedExitCode: expectedErrorCode,
	}
}

func (mcr *mockCmdRunner) Run(options *cmdrunner.RunOptions, format string, vars ...interface{}) (cmdrunner.RunResult, error) {
	if options == nil {
		options = &cmdrunner.RunOptions{}
	}

	return cmdrunner.RunResult{
			ExitCode: mcr.expectedExitCode,
			Output:   common.Redact(options.LogRedactions, mcr.expectedStdout),
			Stderr:   common.Redact(options.LogRedactions, mcr.expectedStderr)},
		nil
}

type CmdClientTestSuite struct {
	suite.Suite
	logger      logger.Logger
	shellClient ShellClient
}

func (suite *CmdClientTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	shellClient, err := NewShellClient(suite.logger, newMockCmdRunner("", "", 0))
	if err != nil {
		panic("Failed to create shell client")
	}

	suite.shellClient = *shellClient
}

func (suite *CmdClientTestSuite) TestShellClientRunContainerReturnsStdout() {
	testPhrase := "testing"
	suite.shellClient.cmdRunner.(*mockCmdRunner).expectedStdout = testPhrase

	output, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})
	suite.Require().NoError(err)

	suite.Equal(testPhrase, output)
}

func (suite *CmdClientTestSuite) TestShellClientRunContainerReturnsMultilineStdout() {
	suite.shellClient.cmdRunner.(*mockCmdRunner).expectedStdout = `
hello world
this is another line
and another
andthisistheid
`

	containerID, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})

	suite.Require().NoError(err)
	suite.Require().Equal("andthisistheid", containerID)
}

func (suite *CmdClientTestSuite) TestShellClientRunContainerReturnsStderr() {
	suite.shellClient.cmdRunner.(*mockCmdRunner).expectedStderr = "foo"

	_, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})

	suite.Require().NoError(err)
}

func (suite *CmdClientTestSuite) TestShellClientRunContainerFailsOnNonSingleStdout() {
	suite.shellClient.cmdRunner.(*mockCmdRunner).expectedStdout = `
hello world
this is another line
and another
andthisistheid with a space`

	_, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})

	suite.Require().Error(err, "Output from docker command includes more than just ID")
}

func (suite *CmdClientTestSuite) TestShellClientRunContainerWhenImageMayNotExist() {
	suite.shellClient.cmdRunner.(*mockCmdRunner).expectedStdout = `
hello world
this is another line
and another
therealidishere
and this a line informing a new version of alpine was pulled. with a space`

	containerID, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports:            map[int]int{7779: 7779},
			ImageMayNotExist: true,
		})

	suite.Require().NoError(err)
	suite.Require().Equal("therealidishere", containerID)
}

func (suite *CmdClientTestSuite) TestShellClientRunContainerRedactsOutput() {
	suite.shellClient.cmdRunner.(*mockCmdRunner).expectedStdout = `helloworldsecret`
	suite.shellClient.redactedValues = append(suite.shellClient.redactedValues, "secret")
	output, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})

	suite.Require().NoError(err)
	suite.Require().Equal("helloworld[redacted]", output)
}

func TestCmdRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(CmdClientTestSuite))
}
