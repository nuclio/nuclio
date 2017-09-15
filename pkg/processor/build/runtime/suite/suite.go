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

package runtimesuite

import (
	"path"
	"os"
	"fmt"
	"net/http"
	"strings"
	"io/ioutil"

	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/zap"
	"github.com/nuclio/nuclio/pkg/dockerclient"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/suite"
	"github.com/rs/xid"
)

type RuntimeTestSuite struct {
	suite.Suite
	Logger nuclio.Logger
	Builder *build.Builder
	DockerClient *dockerclient.Client
	TestID string
}

func (suite *RuntimeTestSuite) SetupSuite() {
	var err error

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.DockerClient, err = dockerclient.NewClient(suite.Logger)
	suite.Require().NoError(err)
}

func (suite *RuntimeTestSuite) SetupTest() {
	suite.TestID = xid.New().String()
}

func (suite *RuntimeTestSuite) BuildAndRunFunction(functionName string,
	functionPath string,
	ports map[int]int,
	requestPort int,
	requestBody string,
	expectedResponseBody string) {

	var err error

	functionName = fmt.Sprintf("%s-%s", functionName, suite.TestID)
	imageName := fmt.Sprintf("nuclio/processor-%s", functionName)

	suite.Builder, err = build.NewBuilder(suite.Logger, &build.Options{
		FunctionName: functionName,
		FunctionPath: functionPath,
		NuclioSourceDir: suite.GetNuclioSourceDir(),
		Verbose: true,
	})

	suite.Require().NoError(err)

	// do the build
	err = suite.Builder.Build()
	suite.Require().NoError(err)

	// remove the image when we're done
	defer suite.DockerClient.RemoveImage(imageName)

	// run the processor
	containerID, err := suite.DockerClient.RunContainer(imageName, ports, "")

	suite.Require().NoError(err)

	// remove the container when we're done
	defer suite.DockerClient.RemoveContainer(containerID)

	// invoke the function
	response, err := http.DefaultClient.Post(fmt.Sprintf("http://localhost:%d", requestPort),
		"text/plain",
		strings.NewReader(requestBody))

	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, response.StatusCode)

	body, err := ioutil.ReadAll(response.Body)
	suite.Require().NoError(err)

	suite.Require().Equal(expectedResponseBody, string(body))
}

func (suite *RuntimeTestSuite) GetNuclioSourceDir() string {
	return path.Join(os.Getenv("GOPATH"), "src", "github.com", "nuclio", "nuclio")
}
