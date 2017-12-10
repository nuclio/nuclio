package dockerclient

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type MockCmdRunner struct {
	mock.Mock
	ExpectedStdOut    string
	ExpectedStdErr    string
	ExpectedErrorCode int
}

func NewMockCmdRunner(expectedStdOut, expectedStdErr string, expectedErrorCode int) *MockCmdRunner {
	return &MockCmdRunner{
		ExpectedStdOut:    expectedStdOut,
		ExpectedStdErr:    expectedStdErr,
		ExpectedErrorCode: expectedErrorCode,
	}
}

func (mcr *MockCmdRunner) Run(options *cmdrunner.RunOptions, format string, vars ...interface{}) (cmdrunner.RunResult, error) {
	return cmdrunner.RunResult{mcr.ExpectedStdOut, mcr.ExpectedStdErr, mcr.ExpectedErrorCode}, nil
}

func (mcr *MockCmdRunner) SetStdOut(s string) {
	mcr.ExpectedStdOut = s
}

type CmdClientTestSuite struct {
	suite.Suite
	logger      nuclio.Logger
	shellClient ShellClient
}

func (suite *CmdClientTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	tempShellClient, err := NewShellClient(suite.logger, NewMockCmdRunner("", "", 0))
	if err != nil {
		panic("Failed to create  shell client")
	}

	suite.shellClient = *tempShellClient
}

func (suite *CmdClientTestSuite) TestShellClientRunContainerReturnsStdOut() {
	testPhrase := "testing"
	suite.shellClient.cmdRunner.(*MockCmdRunner).ExpectedStdOut = testPhrase

	output, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})
	suite.Require().NoError(err)

	suite.Equal(testPhrase, output)
}

func (suite *CmdClientTestSuite) TestShellClientRunContainerFailsOnNonEmptyStdErr() {
	suite.shellClient.cmdRunner.(*MockCmdRunner).ExpectedStdErr = "foo"

	_, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})

	suite.Require().Error(err, "Stderr from docker command is not empty")
}

func (suite *CmdClientTestSuite) TestShellClientRunContainerFailsOnNonSingleStdOut() {
	suite.shellClient.cmdRunner.(*MockCmdRunner).ExpectedStdOut = "hello world"

	_, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})

	suite.Require().Error(err, "Output from docker command includes more than just ID")
}

func TestCmdRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(CmdClientTestSuite))
}
