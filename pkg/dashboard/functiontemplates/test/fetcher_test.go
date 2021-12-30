//go:build test_integration && test_local

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

package test

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type GitFetcherTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *GitFetcherTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
}

func (suite *GitFetcherTestSuite) TestFetch() {
	templateFetcher, err := functiontemplates.NewGitFunctionTemplateFetcher(suite.logger,
		"https://github.com/nuclio/nuclio-templates.git",
		"refs/heads/master",
		"")
	suite.Require().NoError(err)

	templates, err := templateFetcher.Fetch()
	suite.Require().NoError(err)

	suite.logger.DebugWith("Fetcher ended", "templates", templates)
}

func TestGithubFetcher(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(GitFetcherTestSuite))
}

type ZipFetcherTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *ZipFetcherTestSuite) SetupSuite() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
}

func (suite *ZipFetcherTestSuite) TestFetch() {
	templateFetcher, err := functiontemplates.NewZipFunctionTemplateFetcher(suite.logger,
		"https://github.com/nuclio/nuclio-templates/archive/master.zip")

	suite.Require().NoError(err)

	templates, err := templateFetcher.Fetch()
	suite.Require().NoError(err)

	suite.logger.DebugWith("Fetcher ended", "templates", templates)
}

func TestZipFetcher(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(ZipFetcherTestSuite))
}
