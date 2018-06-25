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
	"bufio"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type depedenciesTestSuite struct {
	httpsuite.TestSuite
}

func (suite *depedenciesTestSuite) SetupTest() {
	suite.TestSuite.SetupTest()
	suite.Runtime = "python"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "test", "_functions", "python")
}

func (suite *depedenciesTestSuite) TestDependencies() {

	createFunctionOptions := suite.GetDeployOptions("dependencies",
		suite.GetFunctionPath("dependencies"))

	createFunctionOptions.FunctionConfig.Spec.Handler = "handler:handler"
	expectedResponseBody := suite.getRequestsVersion()
	statusOK := http.StatusOK

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		testRequest := httpsuite.Request{
			ExpectedResponseBody:       expectedResponseBody,
			ExpectedResponseStatusCode: &statusOK,
			RequestPort:                deployResult.Port,
		}

		if !suite.SendRequestVerifyResponse(&testRequest) {
			return false
		}

		return true
	})
}

func (suite *depedenciesTestSuite) getRequestsVersion() string {
	dependenciesFilePath := path.Join(suite.GetFunctionPath("dependencies"), "requirements.txt")

	file, err := os.Open(dependenciesFilePath)
	suite.Require().NoError(err, "Can't open depedencies files")

	var requestsVersion string
	prefix := "requests=="
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, prefix) {
			requestsVersion = line[len(prefix):]
			break
		}
	}

	suite.Require().NoError(scanner.Err(), "Error reading depedencies file")
	suite.Require().NotEmpty(requestsVersion, "Can't find version")

	return requestsVersion
}

func TestDependencies(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, &depedenciesTestSuite{})
}
