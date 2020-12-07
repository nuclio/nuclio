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
	"strings"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ShellClientTestSuite struct {
	suite.Suite
	logger          logger.Logger
	mockedCmdRunner *cmdrunner.MockRunner
	shellClient     *ShellClient
}

func (suite *ShellClientTestSuite) SetupTest() {
	var err error

	// create logger
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Failed to create logger")

	// create mocked cmd runner
	suite.mockedCmdRunner = cmdrunner.NewMockRunner()
	suite.mockedCmdRunner.
		On("Run", mock.Anything, "docker version", mock.Anything).
		Return(cmdrunner.RunResult{
			Output: "test",
		}, nil)

	// create docker shell client
	suite.shellClient, err = NewShellClient(suite.logger, suite.mockedCmdRunner)
	suite.Require().NoError(err, "Failed to create shell client")

	suite.shellClient.buildRetryInterval = 1 * time.Millisecond
}

func (suite *ShellClientTestSuite) TestShellClientRunContainerReturnsStdout() {
	testPhrase := "testing"
	suite.mockedCmdRunner.
		On("Run", mock.Anything, "docker run %s %s %s", mock.Anything).
		Return(cmdrunner.RunResult{
			Output: testPhrase,
		}, nil).
		Once()
	output, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})
	suite.Require().NoError(err)

	suite.Equal(testPhrase, output)
}

func (suite *ShellClientTestSuite) TestShellClientRunContainerReturnsMultilineStdout() {
	suite.mockedCmdRunner.
		On("Run", mock.Anything, "docker run %s %s %s", mock.Anything).
		Return(cmdrunner.RunResult{
			Output: `
hello world
this is another line
and another
andthisistheid
`,
		}, nil).
		Once()

	containerID, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})

	suite.Require().NoError(err)
	suite.Require().Equal("andthisistheid", containerID)
}

func (suite *ShellClientTestSuite) TestShellClientRunContainerReturnsStderr() {
	suite.mockedCmdRunner.
		On("Run", mock.Anything, "docker run %s %s %s", mock.Anything).
		Return(cmdrunner.RunResult{
			Stderr: "foo",
		}, nil).
		Once()
	_, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})

	suite.Require().NoError(err)
}

func (suite *ShellClientTestSuite) TestShellClientRunContainerFailsOnNonSingleStdout() {
	suite.mockedCmdRunner.
		On("Run", mock.Anything, "docker run %s %s %s", mock.Anything).
		Return(cmdrunner.RunResult{
			Output: `
hello world
this is another line
and another
andthisistheid with a space`,
		}, nil).
		Once()
	_, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})

	suite.Require().Error(err, "Output from docker command includes more than just ID")
}

func (suite *ShellClientTestSuite) TestShellClientRunContainerWhenImageMayNotExist() {
	suite.mockedCmdRunner.
		On("Run", mock.Anything, "docker run %s %s %s", mock.Anything).
		Return(cmdrunner.RunResult{
			Output: `
hello world
this is another line
and another
therealidishere
and this a line informing a new version of alpine was pulled. with a space`,
		}, nil).
		Once()

	containerID, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports:            map[int]int{7779: 7779},
			ImageMayNotExist: true,
		})

	suite.Require().NoError(err)
	suite.Require().Equal("therealidishere", containerID)
}

func (suite *ShellClientTestSuite) TestShellClientRunContainerRedactsOutput() {
	suite.mockedCmdRunner.
		On("Run", mock.Anything, "docker run %s %s %s", mock.Anything).
		Return(cmdrunner.RunResult{
			Output: "helloworldsecret",
		}, nil).
		Once()

	suite.shellClient.redactedValues = append(suite.shellClient.redactedValues, "secret")
	output, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})

	suite.Require().NoError(err)
	suite.Require().Equal("helloworld[redacted]", output)
}

func (suite *ShellClientTestSuite) TestBuildBailOnUnknownError() {

	// mock failing
	suite.mockedCmdRunner.
		On("Run",
			mock.Anything,
			mock.MatchedBy(func(command string) bool {
				return strings.Contains(command, "docker build %s")
			}),
			mock.Anything).
		Return(cmdrunner.RunResult{
			Stderr: `some bad happened`,
		}, errors.New("unexpected error")).
		Once()

	err := suite.shellClient.Build(&BuildOptions{
		ContextDir: "",
	})
	suite.Require().Error(err)

	// 1 for docker version + 1 unknown build error
	suite.mockedCmdRunner.AssertNumberOfCalls(suite.T(), "Run", 2)
}

func (suite *ShellClientTestSuite) TestBuildRetryOnErrors() {

	// mock failing
	suite.mockedCmdRunner.
		On("Run",
			mock.Anything,
			mock.MatchedBy(func(command string) bool {
				return strings.Contains(command, "docker build %s")
			}),
			mock.Anything).
		Return(cmdrunner.RunResult{
			Stderr: `Unable to find image 'nuclio-onbuild-someid:sometag' locally`,
		}, errors.New("execution error")).
		Twice()

	// success build
	suite.mockedCmdRunner.
		On("Run", mock.Anything, mock.Anything, mock.Anything).
		Return(cmdrunner.RunResult{}, nil)

	err := suite.shellClient.Build(&BuildOptions{
		ContextDir: "",
	})
	suite.Require().Nil(err)

	// 1 for docker version + 2 failing builds + 1 success build
	suite.mockedCmdRunner.AssertNumberOfCalls(suite.T(), "Run", 4)
}

func TestShellRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(ShellClientTestSuite))
}
