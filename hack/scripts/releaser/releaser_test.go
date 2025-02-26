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

func TestReleaserTestSuite(t *testing.T) {
	suite.Run(t, new(ReleaserTestSuite))
}
