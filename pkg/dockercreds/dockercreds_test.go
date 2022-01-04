//go:build test_unit

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

package dockercreds

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/test/compare"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type DockerCredsTestSuite struct {
	suite.Suite
	logger           logger.Logger
	dockerCreds      *DockerCreds
	mockDockerClient *dockerclient.MockDockerClient
}

func (suite *DockerCredsTestSuite) SetupTest() {
	var err error

	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.mockDockerClient = dockerclient.NewMockDockerClient()
	suite.dockerCreds, err = NewDockerCreds(suite.logger, suite.mockDockerClient, nil)
	suite.Require().NoError(err)
}

//
// Path -> user + URL
//

type GetUserAndURLTestSuite struct {
	DockerCredsTestSuite
}

func (suite *GetUserAndURLTestSuite) TestUserAndURLFromPathSuccessful() {
	user, url, refreshInterval, err := extractMetaFromKeyPath("some-user---some-url.json")
	suite.Require().NoError(err)
	suite.Require().Equal("some-user", user)
	suite.Require().Equal("some-url", url)
	suite.Require().Equal("", refreshInterval)
}

func (suite *GetUserAndURLTestSuite) TestUserAndURLFromPathSuccessfulNoExt() {
	user, url, _, err := extractMetaFromKeyPath("some-user---some-url")
	suite.Require().NoError(err)
	suite.Require().Equal("some-user", user)
	suite.Require().Equal("some-url", url)
}

func (suite *GetUserAndURLTestSuite) TestUserAndURLFromPathNoAt() {
	_, _, _, err := extractMetaFromKeyPath("some-user.json")
	suite.Require().Error(err)
}

func (suite *GetUserAndURLTestSuite) TestUserAndURLFromPathNoUser() {
	_, _, _, err := extractMetaFromKeyPath("---some-url.json")
	suite.Require().Error(err)
	suite.Require().Equal(err.Error(), "Username is empty")
}

func (suite *GetUserAndURLTestSuite) TestUserAndURLFromPathNoURL() {
	_, _, _, err := extractMetaFromKeyPath("some-user---.json")
	suite.Require().Error(err)
	suite.Require().Equal(err.Error(), "URL is empty")
}

func (suite *GetUserAndURLTestSuite) TestUserAndURLFromPathNoUsernameAndURL() {
	_, _, _, err := extractMetaFromKeyPath("---.json")
	suite.Require().Error(err)
}

func (suite *GetUserAndURLTestSuite) TestUserURLAndRefreshIntervalFromPathSuccessful() {
	user, url, refreshInterval, err := extractMetaFromKeyPath("some-user---some-url---10s.json")
	suite.Require().NoError(err)
	suite.Require().Equal("some-user", user)
	suite.Require().Equal("some-url", url)
	suite.Require().Equal("10s", refreshInterval)
}

func (suite *GetUserAndURLTestSuite) TestUserURLAndRefreshIntervalFromPathMissingRefreshInterval() {
	user, url, refreshInterval, err := extractMetaFromKeyPath("some-user---some-url---.json")
	suite.Require().NoError(err)
	suite.Require().Equal("some-user", user)
	suite.Require().Equal("some-url", url)
	suite.Require().Equal("", refreshInterval)
}

//
// Docker registry secrets
//

type ReadKubernetesDockerRegistrySecretTestSuite struct {
	DockerCredsTestSuite
	dockerCred *dockerCred
}

func (suite *ReadKubernetesDockerRegistrySecretTestSuite) SetupTest() {
	suite.DockerCredsTestSuite.SetupTest()

	dockerCreds, _ := NewDockerCreds(suite.logger, nil, nil)
	suite.dockerCred, _ = newDockerCred(dockerCreds, "", nil)
}

func (suite *ReadKubernetesDockerRegistrySecretTestSuite) TestSuccessfulReadAuths() {
	validBody := `{
	"auths": {
		"some-url": { 
			"username": "some-user",
			"password":"some-password",
			"email":"some-email",
			"auth":"dont care"
		}
	}
}`

	secret, err := suite.dockerCred.readKubernetesDockerRegistrySecretAuthsFormat([]byte(validBody))
	suite.Require().NoError(err)

	suite.Equal("some-user", secret.Username)
	suite.Equal("some-password", secret.Password)
	suite.Equal("some-url", secret.URL)
}

func (suite *ReadKubernetesDockerRegistrySecretTestSuite) TestSuccessfulReadNoAuths() {
	validBody := `{
	"some-url": {
		"username": "some-user",
		"password":"some-password",
		"email":"some-email",
		"auth":"dont care"
	}
}`

	secret, err := suite.dockerCred.readKubernetesDockerRegistrySecretNoAuthsFormat([]byte(validBody))
	suite.Require().NoError(err)

	suite.Equal("some-user", secret.Username)
	suite.Equal("some-password", secret.Password)
	suite.Equal("some-url", secret.URL)
}

func (suite *ReadKubernetesDockerRegistrySecretTestSuite) TestInvalidJSONSyntax() {
	invalidBody := `go home JSON, you're drunk'`

	_, err := suite.dockerCred.readKubernetesDockerRegistrySecretAuthsFormat([]byte(invalidBody))
	suite.Require().Error(err)

	_, err = suite.dockerCred.readKubernetesDockerRegistrySecretNoAuthsFormat([]byte(invalidBody))
	suite.Require().Error(err)
}

func (suite *ReadKubernetesDockerRegistrySecretTestSuite) TestTooManyURLsAuths() {
	invalidBody := `{
	"auths": {
		"some-url": { 
			"username": "some-user",
			"password":"some-password",
			"email":"some-email",
			"auth":"dont care"
		},
		"lolz": {}
	}
}`

	_, err := suite.dockerCred.readKubernetesDockerRegistrySecretAuthsFormat([]byte(invalidBody))
	suite.Require().Error(err)
}

func (suite *ReadKubernetesDockerRegistrySecretTestSuite) TestTooManyURLsNoAuths() {
	invalidBody := `{
	"some-url": {
		"username": "some-user",
		"password":"some-password",
		"email":"some-email",
		"auth":"dont care"
	},
	"lolz": {}
}`

	_, err := suite.dockerCred.readKubernetesDockerRegistrySecretNoAuthsFormat([]byte(invalidBody))
	suite.Require().Error(err)
}

//
// Login from dir
//

type fileNode struct {
	name     string
	contents string
}

type dirNode struct {
	name  string
	nodes []interface{}
}

type LogInFromDirTestSuite struct {
	DockerCredsTestSuite
	tempDir                  string
	kubernetesAuthsSecrets   []string
	kubernetesNoAuthsSecrets []string
}

func (suite *LogInFromDirTestSuite) SetupTest() {
	var err error

	suite.DockerCredsTestSuite.SetupTest()

	// create a temp directory
	suite.tempDir, err = ioutil.TempDir("", "dockercreds-test")
	suite.Require().NoError(err)

	// prepare some kubernetes secrets for the tests
	for secretIdx := 0; secretIdx < 3; secretIdx++ {
		secret := fmt.Sprintf(`{
	"auths": {
		"some-url-%d": {
			"username": "some-user-%d",
			"password":"some-password-%d"
		}
	}
}`, secretIdx, secretIdx, secretIdx)

		suite.kubernetesAuthsSecrets = append(suite.kubernetesAuthsSecrets, secret)
	}

	// prepare some kubernetes secrets for the tests
	for secretIdx := 0; secretIdx < 3; secretIdx++ {
		secret := fmt.Sprintf(`{
	"na-some-url-%d": {
		"username": "na-some-user-%d",
		"password":"na-some-password-%d"
	}
}`, secretIdx, secretIdx, secretIdx)

		suite.kubernetesNoAuthsSecrets = append(suite.kubernetesNoAuthsSecrets, secret)
	}
}

func (suite *LogInFromDirTestSuite) TearDownTest() {

	// delete temporary directory
	os.RemoveAll(suite.tempDir)
}

func (suite *LogInFromDirTestSuite) TestLoginSuccessful() {

	err := suite.createFilesInDir(suite.tempDir, []interface{}{
		dirNode{".data", []interface{}{}},
		fileNode{".dockerjsonconfig1", suite.kubernetesAuthsSecrets[0]},
		fileNode{".dockerjsonconfig2", suite.kubernetesAuthsSecrets[1]},
		fileNode{".dockerjsonconfig3", suite.kubernetesNoAuthsSecrets[0]},
		fileNode{"invalid.json", "invalid"},
		fileNode{"user1---url1.json", "pass1"},
		fileNode{"user2---url2.json", "pass2"},
		fileNode{"user3---url3.json", "pass3"},
	})
	suite.Require().NoError(err)

	suite.mockDockerClient.On("LogIn", &dockerclient.LogInOptions{
		Username: "some-user-0",
		Password: "some-password-0",
		URL:      "https://some-url-0",
	}).Return(nil).Once()

	suite.mockDockerClient.On("LogIn", &dockerclient.LogInOptions{
		Username: "some-user-1",
		Password: "some-password-1",
		URL:      "https://some-url-1",
	}).Return(nil).Once()

	suite.mockDockerClient.On("LogIn", &dockerclient.LogInOptions{
		Username: "na-some-user-0",
		Password: "na-some-password-0",
		URL:      "https://na-some-url-0",
	}).Return(nil).Once()

	suite.mockDockerClient.On("LogIn", &dockerclient.LogInOptions{
		Username: "user1",
		Password: "pass1",
		URL:      "https://url1",
	}).Return(nil).Once()

	suite.mockDockerClient.On("LogIn", &dockerclient.LogInOptions{
		Username: "user2",
		Password: "pass2",
		URL:      "https://url2",
	}).Return(nil).Once()

	suite.mockDockerClient.On("LogIn", &dockerclient.LogInOptions{
		Username: "user3",
		Password: "pass3",
		URL:      "https://url3",
	}).Return(nil).Once()

	err = suite.dockerCreds.LoadFromDir(suite.tempDir)
	suite.Require().NoError(err)

	// verify expected credentials
	credentials := suite.dockerCreds.GetCredentials()

	compare.NoOrder(credentials, []Credentials{
		{Username: "some-user-0", Password: "some-password-0", URL: "some-url-0"},
		{Username: "some-user-1", Password: "some-password-1", URL: "some-url-1"},
		{Username: "user1", Password: "pass1", URL: "https://url1"},
		{Username: "user2", Password: "pass2", URL: "https://url2"},
		{Username: "user3", Password: "pass3", URL: "https://url3"},
	})

	// make sure all expectations are met
	suite.mockDockerClient.AssertExpectations(suite.T())
}

func (suite *LogInFromDirTestSuite) TestRefreshLogins() {
	err := suite.createFilesInDir(suite.tempDir, []interface{}{
		fileNode{".dockerjsonconfig1", suite.kubernetesAuthsSecrets[0]},
		fileNode{"user1---url1---1s.json", "pass1"},
		fileNode{"user2---url2.json", "pass2"},
	})
	suite.Require().NoError(err)

	suite.mockDockerClient.On("LogIn", &dockerclient.LogInOptions{
		Username: "some-user-0",
		Password: "some-password-0",
		URL:      "https://some-url-0",
	}).Return(nil).Times(3)

	suite.mockDockerClient.On("LogIn", &dockerclient.LogInOptions{
		Username: "user1",
		Password: "pass1",
		URL:      "https://url1",
	}).Return(nil).Times(4)

	suite.mockDockerClient.On("LogIn", &dockerclient.LogInOptions{
		Username: "user2",
		Password: "pass2",
		URL:      "https://url2",
	}).Return(nil).Times(3)

	defaultRefreshInterval := 1500 * time.Millisecond

	// expect user1 to be refreshed three times (uses interval from name), user 2 to be refreshed twice (uses interval
	// from default). Add one to each since login occurs immediately
	dockerCreds, err := NewDockerCreds(suite.logger, suite.mockDockerClient, &defaultRefreshInterval)
	suite.Require().NoError(err)

	err = dockerCreds.LoadFromDir(suite.tempDir)
	suite.Require().NoError(err)

	// wait 3.5 seconds to allow the 1 second interval to happen fully 3 times
	time.Sleep(3500 * time.Millisecond)

	// make sure all expectations are met
	suite.mockDockerClient.AssertExpectations(suite.T())
}

func (suite *LogInFromDirTestSuite) TestNoRefreshLogins() {
	err := suite.createFilesInDir(suite.tempDir, []interface{}{
		fileNode{"user1---url1.json", "pass1"},
		fileNode{"user2---url2.json", "pass2"},
	})
	suite.Require().NoError(err)

	suite.mockDockerClient.On("LogIn", &dockerclient.LogInOptions{
		Username: "user1",
		Password: "pass1",
		URL:      "https://url1",
	}).Return(nil).Once()

	suite.mockDockerClient.On("LogIn", &dockerclient.LogInOptions{
		Username: "user2",
		Password: "pass2",
		URL:      "https://url2",
	}).Return(nil).Once()

	err = suite.dockerCreds.LoadFromDir(suite.tempDir)
	suite.Require().NoError(err)

	// wait 3 seconds - nothing should happen
	time.Sleep(3 * time.Second)

	// make sure all expectations are met
	suite.mockDockerClient.AssertExpectations(suite.T())
}

func (suite *LogInFromDirTestSuite) createFilesInDir(baseDir string, nodes []interface{}) error {

	for _, node := range nodes {

		switch typedNode := node.(type) {

		// if the node is a file, create it
		case fileNode:
			filePath := path.Join(baseDir, typedNode.name)

			// create the file
			if err := ioutil.WriteFile(filePath, []byte(typedNode.contents), 0644); err != nil {
				return err
			}

		case dirNode:
			dirPath := path.Join(baseDir, typedNode.name)

			if err := os.MkdirAll(dirPath, 0644); err != nil {
				return err
			}

			if err := suite.createFilesInDir(dirPath, typedNode.nodes); err != nil {
				return err
			}
		}
	}

	return nil
}

func TestDockerCredsTestSuite(t *testing.T) {
	suite.Run(t, new(GetUserAndURLTestSuite))
	suite.Run(t, new(ReadKubernetesDockerRegistrySecretTestSuite))
	suite.Run(t, new(LogInFromDirTestSuite))
}
