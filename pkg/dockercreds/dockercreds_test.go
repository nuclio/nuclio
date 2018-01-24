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
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/suite"
)

type DockerCredsTestSuite struct {
	suite.Suite
	logger           nuclio.Logger
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
// Path -> user + URL
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

func (suite *ReadKubernetesDockerRegistrySecretTestSuite) TestSuccessfulRead() {
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

	secret, err := suite.encodeSecretAndRead(validBody)
	suite.Require().NoError(err)

	suite.Equal("some-user", secret.username)
	suite.Equal("some-password", secret.password)
	suite.Equal("some-url", secret.url)
}

func (suite *ReadKubernetesDockerRegistrySecretTestSuite) TestInvalidJSONSyntax() {
	invalidBody := `go home JSON, you're drunk'`
	_, err := suite.encodeSecretAndRead(invalidBody)
	suite.Require().Error(err)
}

func (suite *ReadKubernetesDockerRegistrySecretTestSuite) TestTooManyURLs() {
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

	_, err := suite.encodeSecretAndRead(invalidBody)
	suite.Require().Error(err)
}


func (suite *ReadKubernetesDockerRegistrySecretTestSuite) encodeSecretAndRead(contents string) (secret, error) {
	encodedContents := encodeKubernetesSecret(contents)

	return suite.dockerCred.readKubernetesDockerRegistrySecretFormat([]byte(encodedContents))
}

func encodeKubernetesSecret(contents string) string {
	return base64.StdEncoding.EncodeToString([]byte(contents))
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
	tempDir string
	kubernetesSecrets []string
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

		suite.kubernetesSecrets = append(suite.kubernetesSecrets, encodeKubernetesSecret(secret))
	}
}

func (suite *LogInFromDirTestSuite) TearDownTest() {

	// delete temporary directory
	os.RemoveAll(suite.tempDir)
}

func (suite *LogInFromDirTestSuite) TestLoginSuccessful() {

	suite.createFilesInDir(suite.tempDir, []interface{}{
		fileNode{".dockerjsonconfig1", suite.kubernetesSecrets[0]},
		fileNode{".dockerjsonconfig2", suite.kubernetesSecrets[1]},
		fileNode{"invalid.json", "invalid"},
		fileNode{"user1---url1.json", "pass1"},
		fileNode{"user2---url2.json", "pass2"},
		fileNode{"user3---url3.json", "pass3"},
	})

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

	suite.dockerCreds.LoadFromDir(suite.tempDir)

	// make sure all expectations are met
	suite.mockDockerClient.AssertExpectations(suite.T())
}

func (suite *LogInFromDirTestSuite) TestRefreshLogins() {
	suite.createFilesInDir(suite.tempDir, []interface{}{
		fileNode{".dockerjsonconfig1", suite.kubernetesSecrets[0]},
		fileNode{"user1---url1---1s.json", "pass1"},
		fileNode{"user2---url2.json", "pass2"},
	})

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

	defaultRefreshInterval := time.Duration(1500 * time.Millisecond)

	// expect user1 to be refreshed three times (uses interval from name), user 2 to be refreshed twice (uses interval
	// from default). Add one to each since login occurs immediately
	dockerCreds, err := NewDockerCreds(suite.logger, suite.mockDockerClient, &defaultRefreshInterval)
	suite.Require().NoError(err)

	dockerCreds.LoadFromDir(suite.tempDir)

	// wait 3.5 seconds to allow the 1 second interval to happen fully 3 times
	time.Sleep(3500 * time.Millisecond)

	// make sure all expectations are met
	suite.mockDockerClient.AssertExpectations(suite.T())
}

func (suite *LogInFromDirTestSuite) TestNoRefreshLogins() {
	suite.createFilesInDir(suite.tempDir, []interface{}{
		fileNode{"user1---url1.json", "pass1"},
		fileNode{"user2---url2.json", "pass2"},
	})

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

	suite.dockerCreds.LoadFromDir(suite.tempDir)

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
