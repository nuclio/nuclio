package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/coreos/go-semver/semver"
	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}
func NewTestClient(fn RoundTripFunc) *http.Client { // nolint: interfacer
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
}

func (suite *ReleaserTestSuite) SetupTest() {
	suite.releaser = NewRelease(suite.cmdRunner, suite.logger)
	suite.releaser.targetVersion = &semver.Version{}
	suite.releaser.helmChartsTargetVersion = &semver.Version{}
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

func (suite *ReleaserTestSuite) TestBumpHelmChartVersion() {
	suite.releaser.releaseBranch = "blabla"
	suite.releaser.developmentBranch = "blabla"
	suite.releaser.skipPublishHelmCharts = true

	// checkout to release branch
	suite.cmdRunner.On("Run",
		mock.Anything,
		mock.MatchedBy(func(cmd string) bool {
			return strings.HasPrefix(cmd, `git checkout`)
		}),
		mock.Anything).
		Return(cmdrunner.RunResult{}, nil).
		Once()

	// replace image tag versions X times (e.g.: gke, helm aks)
	suite.cmdRunner.On("Run",
		mock.Anything,
		mock.MatchedBy(func(cmd string) bool {
			return strings.HasPrefix(cmd, "git grep -lF")
		}),
		mock.Anything).
		Return(cmdrunner.RunResult{}, nil).
		Times(len(suite.releaser.resolveSupportedChartDirs()))

	// replace app version
	suite.cmdRunner.On("Run",
		mock.Anything,
		mock.MatchedBy(func(cmd string) bool {
			return strings.Contains(cmd, `\(appVersion: \)`)
		}),
		mock.Anything).
		Return(cmdrunner.RunResult{}, nil).
		Once()

	// replace chart version
	suite.cmdRunner.On("Run",
		mock.Anything,
		mock.MatchedBy(func(cmd string) bool {
			return strings.Contains(cmd, `\(version: \)`)
		}),
		mock.Anything).
		Return(cmdrunner.RunResult{}, nil).
		Once()

	// commit
	suite.cmdRunner.On("Run",
		mock.Anything,
		mock.MatchedBy(func(cmd string) bool {
			return strings.HasPrefix(cmd, `git commit`)
		}),
		mock.Anything).
		Return(cmdrunner.RunResult{}, nil).
		Once()

	// push
	suite.cmdRunner.On("Run",
		mock.Anything,
		mock.MatchedBy(func(cmd string) bool {
			return strings.HasPrefix(cmd, `git push`)
		}),
		mock.Anything).
		Return(cmdrunner.RunResult{}, nil).
		Once()

	err := suite.releaser.bumpHelmChartVersion()
	suite.Require().NoError(err)

	suite.cmdRunner.AssertExpectations(suite.T())
}

func (suite *ReleaserTestSuite) TestResolveDesiredPatchVersions() {
	suite.releaser.helmChartConfig = helmChart{
		Version: semver.Version{
			Patch: 1,
		},
		AppVersion: semver.Version{
			Patch: 2,
		},
	}
	err := suite.releaser.resolveDesiredPatchVersion()
	suite.Require().NoError(err)

	suite.Require().Equal(suite.releaser.helmChartConfig.AppVersion.Patch+1,
		suite.releaser.targetVersion.Patch)
	suite.Require().Equal(suite.releaser.helmChartConfig.Version.Patch+1,
		suite.releaser.helmChartsTargetVersion.Patch)

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
