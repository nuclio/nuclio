//go:build test_unit

/*
Copyright 2026 The Nuclio Authors.

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

package git

import (
	"bytes"
	"testing"

	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type CloneValidationTestSuite struct {
	suite.Suite
}

// TestValidateRepositoryURLAcceptsLegitimateURLs guards against a future tightening of
// validateRepositoryURL breaking any git provider/URL shape this repo already supports.
func (suite *CloneValidationTestSuite) TestValidateRepositoryURLAcceptsLegitimateURLs() {
	for _, testCase := range []struct {
		name          string
		repositoryURL string
	}{
		{name: "GitHubHTTPSWithGitSuffix", repositoryURL: "https://github.com/user92/test-nuclio-cet.git"},
		{name: "BitBucketHTTPSWithGitSuffix", repositoryURL: "https://bitbucket.org/user/test-nuclio-cet.git"},
		{name: "BitBucketHTTPSWithoutGitSuffix", repositoryURL: "https://bitbucket.org/my-user/my-repo"},
		{name: "AzureDevOpsHTTPS", repositoryURL: "https://dev.azure.com/user920089/test-nuclio-cet/_git/test-nuclio-cet"},
		{
			name:          "AzureDevOpsHTTPSWithEmbeddedUserinfo",
			repositoryURL: "https://user920089@dev.azure.com/user920089/test-nuclio-cet/_git/test-nuclio-cet",
		},
		{name: "SelfHostedGitServerHTTPS", repositoryURL: "https://git.example.com/my-user/my-repo"},
		{name: "PlainHTTP", repositoryURL: "http://github.com/my-org/my-repo"},
	} {
		suite.Run(testCase.name, func() {
			suite.Require().NoError(validateRepositoryURL(testCase.repositoryURL, "refs/heads/main"))
		})
	}
}

func (suite *CloneValidationTestSuite) TestValidateRepositoryURLRejectsMaliciousInput() {
	for _, testCase := range []struct {
		name          string
		repositoryURL string
		referenceName string
	}{
		{name: "CRLFInjection", repositoryURL: "https://github.com/org/repo.git\r\n[FAKE] admin login", referenceName: "refs/heads/main"},
		{name: "ANSIEscapeInjection", repositoryURL: "https://github.com/org/repo.git\x1b[31mFAKE\x1b[0m", referenceName: "refs/heads/main"},
		{name: "MalformedPercentEncoding", repositoryURL: "https://github.com/org/repo.git%zz", referenceName: "refs/heads/main"},
		{name: "MaliciousReferenceName", repositoryURL: "https://github.com/org/repo.git", referenceName: "refs/heads/main\n2024 ERROR fake"},
	} {
		suite.Run(testCase.name, func() {
			suite.Require().Error(validateRepositoryURL(testCase.repositoryURL, testCase.referenceName))
		})
	}
}

// TestContainsControlCharactersAcceptsLegitimateReferenceNames guards against a future
// tightening of containsControlCharacters breaking any branch/tag/reference shape this repo
// already supports (see builder_test.go's TestResolveFunctionPathGitCodeEntry).
func (suite *CloneValidationTestSuite) TestContainsControlCharactersAcceptsLegitimateReferenceNames() {
	for _, testCase := range []struct {
		name          string
		referenceName string
	}{
		{name: "Branch", referenceName: "refs/heads/go-func"},
		{name: "BranchWithSlash", referenceName: "refs/heads/feature/foo"},
		{name: "Tag", referenceName: "refs/tags/0.0.1"},
		{name: "FullReference", referenceName: "refs/heads/go-func"},
		{name: "PercentSignNotPercentEncoded", referenceName: "refs/heads/release%2024"},
	} {
		suite.Run(testCase.name, func() {
			suite.Require().False(containsControlCharacters(testCase.referenceName))
		})
	}
}

func (suite *CloneValidationTestSuite) TestContainsControlCharactersRejectsMaliciousInput() {
	for _, testCase := range []struct {
		name          string
		referenceName string
	}{
		{name: "NewlineInjection", referenceName: "refs/heads/main\n2024 ERROR fake"},
		{name: "ANSIEscapeInjection", referenceName: "refs/heads/main\x1b[31mFAKE\x1b[0m"},
	} {
		suite.Run(testCase.name, func() {
			suite.Require().True(containsControlCharacters(testCase.referenceName))
		})
	}
}

// TestCloneLogsLegitimateInputAndRejectsMaliciousInput proves malicious input is rejected before
// the "Cloning" log line, while every legitimate case still reaches that log line unmodified.
func (suite *CloneValidationTestSuite) TestCloneLogsLegitimateInputAndRejectsMaliciousInput() {
	const unreachableRepositoryURL = "https://127.0.0.1:1/test-org/test-repo.git"

	// every legitimate case must not be blocked by our own validation, whatever happens
	// afterward when git.PlainClone dials the (deliberately unreachable) host
	validationErrors := []string{"Failed to validate git clone inputs", "contains control characters"}

	for _, testCase := range []struct {
		name            string
		repositoryURL   string
		attributes      *Attributes
		wantErrContains string // set only for cases validation must reject
		wantLogContains []string
		wantLogAbsent   []string // injected payloads that must never reach the log
	}{
		{
			name:            "MaliciousRepositoryURL",
			repositoryURL:   "https://github.com/org/repo.git\r\n[FAKE] admin login",
			attributes:      &Attributes{Branch: "main"},
			wantErrContains: "Failed to validate git clone inputs",
			wantLogAbsent:   []string{"[FAKE]"},
		},
		{
			name:            "MaliciousReferenceName",
			repositoryURL:   "https://github.com/org/repo.git",
			attributes:      &Attributes{Branch: "main\n2024 ERROR fake"},
			wantErrContains: "Failed to validate git clone inputs",
			wantLogAbsent:   []string{"ERROR fake"},
		},
		{
			name:            "LegitimateRepositoryURL",
			repositoryURL:   unreachableRepositoryURL,
			attributes:      &Attributes{Branch: "main"},
			wantLogContains: []string{unreachableRepositoryURL, "refs/heads/main"},
		},
		{
			name:            "LegitimateBranchReferenceName",
			repositoryURL:   unreachableRepositoryURL,
			attributes:      &Attributes{Branch: "go-func"},
			wantLogContains: []string{"refs/heads/go-func"},
		},
		{
			name:            "LegitimateTagReferenceName",
			repositoryURL:   unreachableRepositoryURL,
			attributes:      &Attributes{Tag: "0.0.1"},
			wantLogContains: []string{"refs/tags/0.0.1"},
		},
		{
			name:            "LegitimateFullReferenceName",
			repositoryURL:   unreachableRepositoryURL,
			attributes:      &Attributes{Reference: "refs/heads/go-func"},
			wantLogContains: []string{"refs/heads/go-func"},
		},
	} {
		suite.Run(testCase.name, func() {
			var logBuf bytes.Buffer
			captureLogger, err := nucliozap.NewNuclioZap("test", "json", nil, &logBuf, &logBuf, nucliozap.DebugLevel)
			suite.Require().NoError(err)

			abstractClient := &AbstractClient{logger: captureLogger}

			err = abstractClient.Clone(suite.T().TempDir(), testCase.repositoryURL, testCase.attributes)
			suite.Require().Error(err) // malicious: rejected by validation; legitimate: connection refused after logging

			if testCase.wantErrContains != "" {
				suite.Contains(err.Error(), testCase.wantErrContains)
			} else {
				for _, validationErr := range validationErrors {
					suite.NotContains(err.Error(), validationErr)
				}
			}

			logLine := logBuf.String()
			for _, substr := range testCase.wantLogContains {
				suite.Contains(logLine, substr)
			}
			for _, substr := range testCase.wantLogAbsent {
				suite.NotContains(logLine, substr)
			}
		})
	}
}

func TestCloneValidationTestSuite(t *testing.T) {
	suite.Run(t, new(CloneValidationTestSuite))
}
