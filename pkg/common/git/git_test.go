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

package git

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	cryptossh "golang.org/x/crypto/ssh"
)

type AuthMethodTestSuite struct {
	suite.Suite
	logger logger.Logger
	client *AbstractClient
}

const testKnownHosts = "github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl\n"

func (suite *AuthMethodTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.client = &AbstractClient{logger: suite.logger}
}

// generateTestSSHPrivateKey returns a freshly-generated, PEM-encoded OpenSSH private key
func (suite *AuthMethodTestSuite) generateTestSSHPrivateKey() string {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	suite.Require().NoError(err)

	pemBlock, err := cryptossh.MarshalPrivateKey(privateKey, "")
	suite.Require().NoError(err)

	return string(pem.EncodeToMemory(pemBlock))
}

func (suite *AuthMethodTestSuite) TestNoCredentialsReturnsNilAuth() {
	authMethod, err := suite.client.resolveAuthMethod("https://github.com/org/repo", &Attributes{})
	suite.Require().NoError(err)
	suite.Require().Nil(authMethod)
}

func (suite *AuthMethodTestSuite) TestHTTPBasicAuth() {
	authMethod, err := suite.client.resolveAuthMethod("https://github.com/org/repo", &Attributes{
		Username: "user",
		Password: "token",
	})
	suite.Require().NoError(err)

	basicAuth, ok := authMethod.(*githttp.BasicAuth)
	suite.Require().True(ok, "expected HTTP basic auth")
	suite.Require().Equal("user", basicAuth.Username)
	suite.Require().Equal("token", basicAuth.Password)
}

func (suite *AuthMethodTestSuite) TestHTTPBasicAuthDefaultUsername() {
	// when only a password (e.g. a PAT) is given, a non-empty username is filled in
	authMethod, err := suite.client.resolveAuthMethod("https://github.com/org/repo", &Attributes{
		Password: "token",
	})
	suite.Require().NoError(err)

	basicAuth, ok := authMethod.(*githttp.BasicAuth)
	suite.Require().True(ok)
	suite.Require().NotEmpty(basicAuth.Username)
}

func (suite *AuthMethodTestSuite) TestSSHAuthFromPrivateKey() {
	authMethod, err := suite.client.resolveAuthMethod("ssh://git@github.com/org/repo.git", &Attributes{
		SSHPrivateKey: suite.generateTestSSHPrivateKey(),
		SSHKnownHosts: testKnownHosts,
	})
	suite.Require().NoError(err)

	publicKeys, ok := authMethod.(*gitssh.PublicKeys)
	suite.Require().True(ok, "expected SSH public key auth")
	suite.Require().Equal("git", publicKeys.User)

	suite.Require().NotNil(publicKeys.HostKeyCallback)
}

func (suite *AuthMethodTestSuite) TestSSHAuthWithoutKnownHostsFails() {
	_, err := suite.client.resolveAuthMethod("ssh://git@github.com/org/repo.git", &Attributes{
		SSHPrivateKey: suite.generateTestSSHPrivateKey(),
	})
	suite.Require().Error(err)
}

func (suite *AuthMethodTestSuite) TestSSHCredentialsWithHTTPURLFails() {
	_, err := suite.client.resolveAuthMethod("https://github.com/org/repo", &Attributes{
		SSHPrivateKey: suite.generateTestSSHPrivateKey(),
	})
	suite.Require().Error(err)
	suite.Require().Contains(err.Error(), "require an SSH repository URL")
}

func (suite *AuthMethodTestSuite) TestSSHAuthInferredFromURL() {
	// an scp-like SSH URL selects SSH auth even when SSHPrivateKey was not explicitly checked first
	authMethod, err := suite.client.resolveAuthMethod("git@github.com:org/repo.git", &Attributes{
		SSHPrivateKey: suite.generateTestSSHPrivateKey(),
		SSHKnownHosts: testKnownHosts,
	})
	suite.Require().NoError(err)

	_, ok := authMethod.(*gitssh.PublicKeys)
	suite.Require().True(ok)
}

func (suite *AuthMethodTestSuite) TestSSHUserFromURL() {
	authMethod, err := suite.client.resolveAuthMethod("ssh://builder@github.com/org/repo.git", &Attributes{
		SSHPrivateKey: suite.generateTestSSHPrivateKey(),
		SSHKnownHosts: testKnownHosts,
	})
	suite.Require().NoError(err)

	publicKeys, ok := authMethod.(*gitssh.PublicKeys)
	suite.Require().True(ok)
	suite.Require().Equal("builder", publicKeys.User)
}

func (suite *AuthMethodTestSuite) TestSSHURLWithoutKeyFails() {
	// an SSH URL with no private key is a bad request
	_, err := suite.client.resolveAuthMethod("git@github.com:org/repo.git", &Attributes{})
	suite.Require().Error(err)
}

func (suite *AuthMethodTestSuite) TestSSHAuthWithKnownHosts() {
	authMethod, err := suite.client.resolveAuthMethod("git@github.com:org/repo.git", &Attributes{
		SSHPrivateKey: suite.generateTestSSHPrivateKey(),
		SSHKnownHosts: testKnownHosts,
	})
	suite.Require().NoError(err)

	publicKeys, ok := authMethod.(*gitssh.PublicKeys)
	suite.Require().True(ok)
	suite.Require().NotNil(publicKeys.HostKeyCallback)
}

func (suite *AuthMethodTestSuite) TestIsSSHRepositoryURL() {
	for _, testCase := range []struct {
		url      string
		expected bool
	}{
		{"ssh://git@github.com/org/repo.git", true},
		{"SSH://git@github.com/org/repo.git", true},
		{"  ssh://git@github.com/org/repo.git  ", true},
		{"git@github.com:org/repo.git", true},
		{"git@ssh.dev.azure.com:v3/org/project/repo", true},
		{"https://github.com/org/repo.git", false},
		{"http://github.com/org/repo.git", false},
		{"https://user@github.com/org/repo.git", false},
	} {
		suite.Require().Equal(testCase.expected, IsSSHRepositoryURL(testCase.url), "url: %s", testCase.url)
	}
}

func TestResolveReferenceForAzureDevOpsSSH(t *testing.T) {
	reference, err := ResolveReference("git@ssh.dev.azure.com:v3/org/project/repo", &Attributes{Branch: "main"})
	require.NoError(t, err)
	require.Equal(t, "refs/heads/main", reference)
}

func TestAuthMethodTestSuite(t *testing.T) {
	suite.Run(t, new(AuthMethodTestSuite))
}
