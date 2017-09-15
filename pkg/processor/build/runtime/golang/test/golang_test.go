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
	"path"
	"os"

	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/suite"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/rs/xid"
	"fmt"
	"net/http"
	"strings"
	"io/ioutil"
)

type GolangBuildTestSuite struct {
	suite.Suite
	logger nuclio.Logger
	builder *build.Builder
	dockerClient *dockerclient.Client
	testID string
}

func (suite *GolangBuildTestSuite) SetupSuite() {
	var err error

	suite.logger, err = nucliozap.NewNuclioZapTest("golang_build")
	suite.Require().NoError(err)

	suite.dockerClient, err = dockerclient.NewClient(suite.logger)
	suite.Require().NoError(err)
}

func (suite *GolangBuildTestSuite) SetupTest() {
	suite.testID = xid.New().String()
}

func (suite *GolangBuildTestSuite) TestBuildFileNoRuntime() {
	var err error

	functionName := fmt.Sprintf("incrementor-%s", suite.testID)
	imageName := fmt.Sprintf("nuclio/processor-%s", functionName)

	suite.builder, err = build.NewBuilder(suite.logger, &build.Options{
		FunctionName: functionName,
		FunctionPath: path.Join(suite.getGolangRuntimeDir(), "test", "incrementor", "incrementor.go"),
		NuclioSourceDir: suite.getNuclioSourceDir(),
		Verbose: true,
	})

	suite.Require().NoError(err)

	// do the build
	err = suite.builder.Build()
	suite.Require().NoError(err)

	// remove the image when we're done
	defer suite.dockerClient.RemoveImage(imageName)

	// run the processor
	containerID, err := suite.dockerClient.RunContainer(imageName, map[int]int{8080: 8080}, "")

	suite.Require().NoError(err)

	// remove the container when we're done
	defer suite.dockerClient.RemoveContainer(containerID)

	// invoke the function
	response, err := http.DefaultClient.Post("http://localhost:8080",
		"text/plain",
		strings.NewReader("abcdef"))

	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, response.StatusCode)

	body, err := ioutil.ReadAll(response.Body)
	suite.Require().NoError(err)

	suite.Require().Equal("bcdefg", string(body))
}

func (suite *GolangBuildTestSuite) getGolangRuntimeDir() string {
	return path.Join(suite.getNuclioSourceDir(), "pkg", "processor", "build", "runtime", "golang")
}

func (suite *GolangBuildTestSuite) getNuclioSourceDir() string {
	return path.Join(os.Getenv("GOPATH"), "src", "github.com", "nuclio", "nuclio")
}

func TestGolangBuildTestSuite(t *testing.T) {
	suite.Run(t, new(GolangBuildTestSuite))
}
