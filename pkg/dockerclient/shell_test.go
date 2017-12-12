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
	expectedStdOut   string
	expectedStdErr   string
	expectedExitCode int
}

func NewMockCmdRunner(expectedStdOut, expectedStdErr string, expectedErrorCode int) *mockCmdRunner {
	return &mockCmdRunner{
		expectedStdOut:   expectedStdOut,
		expectedStdErr:   expectedStdErr,
		expectedExitCode: expectedErrorCode,
	}
}

func (mcr *mockCmdRunner) Run(options *cmdrunner.RunOptions, format string, vars ...interface{}) (cmdrunner.RunResult, error) {
	return cmdrunner.RunResult{mcr.expectedStdOut, mcr.expectedStdErr, mcr.expectedExitCode}, nil
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

func (suite *CmdClientTestSuite) TestShellClientRunContainerReturnsStdOut() {
	testPhrase := "testing"
	suite.shellClient.cmdRunner.(*mockCmdRunner).expectedStdOut = testPhrase

	output, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})
	suite.Require().NoError(err)

	suite.Equal(testPhrase, output)
}

func (suite *CmdClientTestSuite) TestShellClientRunContainerFailsOnNonEmptyStdErr() {
	suite.shellClient.cmdRunner.(*mockCmdRunner).expectedStdErr = "foo"

	_, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})

	suite.Require().Error(err, "Stderr from docker command is not empty")
}

func (suite *CmdClientTestSuite) TestShellClientRunContainerFailsOnNonSingleStdOut() {
	suite.shellClient.cmdRunner.(*mockCmdRunner).expectedStdOut = "hello world"

	_, err := suite.shellClient.RunContainer("alpine",
		&RunOptions{
			Ports: map[int]int{7779: 7779},
		})

	suite.Require().Error(err, "Output from docker command includes more than just ID")
}

func TestCmdRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(CmdClientTestSuite))
}
