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
	"io/ioutil"
	"os"
	"path"
	"testing"

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
	suite.dockerCreds, err = NewDockerCreds(suite.logger, suite.mockDockerClient)
	suite.Require().NoError(err)
}

//
// Path -> user + URL
//

type GetUserAndURLTestSuite struct {
	DockerCredsTestSuite
}

func (suite *GetUserAndURLTestSuite) TestUserAndURLFromPathSuccessful() {
	user, url, err := suite.dockerCreds.getUserAndURLFromKeyPath("some-user---some-url.json")
	suite.Require().NoError(err)
	suite.Require().Equal("some-user", user)
	suite.Require().Equal("some-url", url)
}

func (suite *GetUserAndURLTestSuite) TestUserAndURLFromPathSuccessfulNoExt() {
	user, url, err := suite.dockerCreds.getUserAndURLFromKeyPath("some-user---some-url")
	suite.Require().NoError(err)
	suite.Require().Equal("some-user", user)
	suite.Require().Equal("some-url", url)
}

func (suite *GetUserAndURLTestSuite) TestUserAndURLFromPathNoAt() {
	_, _, err := suite.dockerCreds.getUserAndURLFromKeyPath("some-user.json")
	suite.Require().Error(err)
}

func (suite *GetUserAndURLTestSuite) TestUserAndURLFromPathNoUser() {
	_, _, err := suite.dockerCreds.getUserAndURLFromKeyPath("---some-url.json")
	suite.Require().Error(err)
	suite.Require().Equal(err.Error(), "Username is empty")
}

func (suite *GetUserAndURLTestSuite) TestUserAndURLFromPathNoURL() {
	_, _, err := suite.dockerCreds.getUserAndURLFromKeyPath("some-user---.json")
	suite.Require().Error(err)
	suite.Require().Equal(err.Error(), "URL is empty")
}

func (suite *GetUserAndURLTestSuite) TestUserAndURLFromPathNoUsernameAndURL() {
	_, _, err := suite.dockerCreds.getUserAndURLFromKeyPath("---.json")
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
	tempDir string
}

func (suite *LogInFromDirTestSuite) SetupTest() {
	var err error

	suite.DockerCredsTestSuite.SetupTest()

	// create a temp directory
	suite.tempDir, err = ioutil.TempDir("", "loginner-test")
	suite.Require().NoError(err)
}

func (suite *LogInFromDirTestSuite) TearDownTest() {

	// delete temporary directory
	os.RemoveAll(suite.tempDir)
}

func (suite *LogInFromDirTestSuite) TestLoginSuccessful() {
	suite.createFilesInDir(suite.tempDir, []interface{}{
		fileNode{"user1---url1.json", "pass1"},
		fileNode{"user2---url2.json", "pass2"},
		fileNode{"invalid.json", "invalid"},
		fileNode{"user3---url3.json", "pass3"},
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

	suite.mockDockerClient.On("LogIn", &dockerclient.LogInOptions{
		Username: "user3",
		Password: "pass3",
		URL:      "https://url3",
	}).Return(nil).Once()

	suite.dockerCreds.LoadFromDir(suite.tempDir)

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
	suite.Run(t, new(LogInFromDirTestSuite))
}
