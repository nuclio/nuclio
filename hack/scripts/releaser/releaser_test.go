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
	"net/http"
	"os"
	"testing"

	"github.com/nuclio/nuclio/pkg/cmdrunner"

	"github.com/coreos/go-semver/semver"
	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
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

func (suite *ReleaserTestSuite) TestSaveReleaseInfo() {
	// Create a temporary file to save the release info
	tempFile, err := os.CreateTemp("", "release_info_*.txt")
	suite.Require().NoError(err)
	defer os.Remove(tempFile.Name())

	// Set the release info path to the temporary file
	suite.releaser.releaseInfoPath = tempFile.Name()

	// Set the versions to be saved
	suite.releaser.currentVersion = semver.New("1.0.0")
	suite.releaser.targetVersion = semver.New("1.1.0")
	suite.releaser.helmChartsTargetVersion = semver.New("1.1.0")

	// Call the saveReleaseInfo method
	err = suite.releaser.saveReleaseInfo()
	suite.Require().NoError(err)

	// Read the contents of the temporary file
	content, err := os.ReadFile(tempFile.Name())
	suite.Require().NoError(err)

	// Verify the contents of the file
	expectedContent := "CURRENT_VERSION: 1.0.0\nTARGET_VERSION: 1.1.0\nHELM_CHARTS_TARGET_VERSION: 1.1.0\n"
	suite.Require().Equal(expectedContent, string(content))
}

func TestReleaserTestSuite(t *testing.T) {
	suite.Run(t, new(ReleaserTestSuite))
}
