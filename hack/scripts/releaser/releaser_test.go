package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"runtime"
	"testing"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}
func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

type ReleaserTestSuite struct {
	suite.Suite
	releaser  *Release
	cmdRunner *cmdrunner.MockRunner
	logger    logger.Logger
}

func (suite *ReleaserTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
	suite.cmdRunner = cmdrunner.NewMockRunner()
	suite.releaser = NewRelease(suite.cmdRunner, suite.logger)
	suite.Require().NoError(err)
}

func (suite *ReleaserTestSuite) TestCreateReleaseCopyToClipboard() {
	if runtime.GOOS != "darwin" {
		suite.Suite.T().Skipf("This test uses macOS 'pbcopy' command. %s != darwin", runtime.GOOS)
	}
	randomReleaseNotes := common.GenerateRandomString(5000, common.LettersAndNumbers)

	// get release notes
	suite.cmdRunner.
		On("Run",
			mock.Anything,
			mock.Anything,
			mock.Anything).
		Return(cmdrunner.RunResult{
			Output: randomReleaseNotes,
		}, nil).
		Once()

	// open window
	suite.cmdRunner.
		On("Run",
			mock.Anything,
			mock.Anything,
			mock.Anything).
		Return(cmdrunner.RunResult{}, nil).
		Once()

	suite.mockHTTPClientResponses([]string{

		// get workflows
		`{"workflows": [{"id": 123, "name": "Release"}]}`,

		// get workflow first status
		`{"workflow_runs": [{"status": "completed", "conclusion": "success"}]}`,
	})

	err := suite.releaser.createRelease()
	suite.Require().NoError(err)
}

func (suite *ReleaserTestSuite) mockHTTPClientResponses(responses []string) {
	http.DefaultClient = NewTestClient(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: 200,
			Body: func() io.ReadCloser {
				responseBody := responses[0]
				responses = responses[1:]
				return ioutil.NopCloser(bytes.NewBufferString(responseBody))
			}(),
			Header: make(http.Header),
		}
	})
}

func TestReleaserTestSuite(t *testing.T) {
	suite.Run(t, new(ReleaserTestSuite))
}
