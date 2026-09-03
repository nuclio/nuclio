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

// TestContainsControlCharacters guards against a future tightening of containsControlCharacters
// breaking any branch/tag/reference shape this repo already supports (see builder_test.go's
// TestResolveFunctionPathGitCodeEntry), while confirming it still rejects control characters.
func (suite *CloneValidationTestSuite) TestContainsControlCharacters() {
	for _, testCase := range []struct {
		name          string
		referenceName string
		expected      bool
	}{
		// happy flows
		{name: "Branch", referenceName: "refs/heads/go-func"},
		{name: "BranchWithSlash", referenceName: "refs/heads/feature/foo"},
		{name: "Tag", referenceName: "refs/tags/0.0.1"},
		{name: "FullReference", referenceName: "refs/heads/go-func"},
		{name: "PercentSignNotPercentEncoded", referenceName: "refs/heads/release%2024"},

		// malicious inputs
		{name: "NewlineInjection", referenceName: "refs/heads/main\n2024 ERROR fake", expected: true},
		{name: "ANSIEscapeInjection", referenceName: "refs/heads/main\x1b[31mFAKE\x1b[0m", expected: true},
	} {
		suite.Run(testCase.name, func() {
			suite.Equal(testCase.expected, containsControlCharacters(testCase.referenceName))
		})
	}
}

// TestCloneLogsLegitimateInputAndRejectsMaliciousInput proves malicious input is rejected by
// validateRepositoryURL before Clone ever calls the logger, while every legitimate case still
// reaches the "Cloning" log line unmodified. Legitimate cases target 127.0.0.1:1 (nothing listens
// there) so Clone still errors afterward on the refused connection - that failure is expected and
// irrelevant here; what matters is that it happens only after validation passed and logging ran.
func (suite *CloneValidationTestSuite) TestCloneLogsLegitimateInputAndRejectsMaliciousInput() {
	const unreachableRepositoryURL = "https://127.0.0.1:1/test-org/test-repo.git"

	for _, testCase := range []struct {
		name            string
		repositoryURL   string
		attributes      *Attributes
		malicious       bool
		wantLogContains []string
	}{
		// malicious inputs: rejected before anything is logged
		{
			name:          "MaliciousRepositoryURL",
			repositoryURL: "https://github.com/org/repo.git\r\n[FAKE] admin login",
			attributes:    &Attributes{Branch: "main"},
			malicious:     true,
		},
		{
			name:          "MaliciousReferenceName",
			repositoryURL: "https://github.com/org/repo.git",
			attributes:    &Attributes{Branch: "main\n2024 ERROR fake"},
			malicious:     true,
		},

		// happy flows: pass validation and reach the "Cloning" log line unmodified
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
			suite.Require().Error(err)

			if testCase.malicious {
				suite.Contains(err.Error(), "Failed to validate git clone inputs")
				suite.Empty(logBuf.String(), "malicious input must be rejected before anything is logged")
				return
			}

			logLine := logBuf.String()
			for _, substr := range testCase.wantLogContains {
				suite.Contains(logLine, substr)
			}
		})
	}
}

func TestCloneValidationTestSuite(t *testing.T) {
	suite.Run(t, new(CloneValidationTestSuite))
}
