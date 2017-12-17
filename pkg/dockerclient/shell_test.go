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
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type mockCmdRunner struct {
	mock.Mock
	expectedStdout   string
	expectedStderr   string
	expectedExitCode int
}

func NewMockCmdRunner(expectedStdout, expectedStderr string, expectedErrorCode int) *mockCmdRunner {
	return &mockCmdRunner{
		expectedStdout:   expectedStdout,
		expectedStderr:   expectedStderr,
		expectedExitCode: expectedErrorCode,
	}
}

func (mcr *mockCmdRunner) Run(options *cmdrunner.RunOptions, format string, vars ...interface{}) (cmdrunner.RunResult, error) {
	return cmdrunner.RunResult{mcr.expectedStdout, mcr.expectedStderr, mcr.expectedExitCode}, nil
}

type CmdClientTestSuite struct {
	suite.Suite
	logger      nuclio.Logger
	shellClient ShellClient
}

func (suite *CmdClientTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	shellClient, err := NewShellClient(suite.logger, NewMockCmdRunner("", "", 0))
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

func TestCmdRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(CmdClientTestSuite))
}
