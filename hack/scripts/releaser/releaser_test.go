//go:build test_unit

/*
Copyright 2023 The Nuclio Authors.

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

package main

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/cmdrunner"
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

func (suite *ReleaserTestSuite) TestBumpHelmChartVersion() {
	suite.releaser.releaseBranch = "x.y.z"
	suite.releaser.developmentBranch = "x.y.z"
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

	// replace image tag versions
	suite.cmdRunner.On("Run",
		mock.Anything,
		mock.MatchedBy(func(cmd string) bool {
			return strings.HasPrefix(cmd, "git grep -lF")
		}),
		mock.Anything).
		Return(cmdrunner.RunResult{}, nil)

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

	// status
	suite.cmdRunner.On("Run",
		mock.Anything,
		mock.MatchedBy(func(cmd string) bool {
			return strings.HasPrefix(cmd, `git status`)
		}),
		mock.Anything).
		Return(cmdrunner.RunResult{
			Output: "M helm/Chart.yaml\nM helm/values.yaml",
		}, nil).
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
	suite.releaser.bumpPatch = true
	suite.releaser.helmChartConfig = helmChart{
		Version: semver.Version{
			Patch: 1,
		},
		AppVersion: semver.Version{
			Patch: 2,
		},
	}
	err := suite.releaser.populateBumpedVersions()
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
				return io.NopCloser(bytes.NewBufferString(responseBody))
			}(),
			Header: make(http.Header),
		}
	})
}

func TestReleaserTestSuite(t *testing.T) {
	suite.Run(t, new(ReleaserTestSuite))
}
