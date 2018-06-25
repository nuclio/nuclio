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
	"encoding/json"
	"net/http"
	"os"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type packagesFile struct {
	Dependencies map[string]string `json:"dependencies"`
}

type depedenciesTestSuite struct {
	httpsuite.TestSuite
}

func (suite *depedenciesTestSuite) SetupTest() {
	suite.TestSuite.SetupTest()
	suite.Runtime = "nodejs"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "test", "_functions", "nodejs")
}

func (suite *depedenciesTestSuite) TestDependencies() {

	createFunctionOptions := suite.GetDeployOptions("dependencies",
		suite.GetFunctionPath("dependencies"))

	createFunctionOptions.FunctionConfig.Spec.Handler = "handler:handler"
	expectedResponseBody := suite.getMomentVersion()
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

func (suite *depedenciesTestSuite) getMomentVersion() string {
	dependenciesFilePath := path.Join(suite.GetFunctionPath("dependencies"), "package.json")

	file, err := os.Open(dependenciesFilePath)
	suite.Require().NoError(err, "Can't open depedencies files")

	var packages packagesFile
	err = json.NewDecoder(file).Decode(&packages)
	suite.Require().NoError(err, "Can't read depedencies file as JSON")
	suite.Require().NotEmpty(packages.Dependencies, "No dependencies found")

	var momentVersion string

	for name, version := range packages.Dependencies {
		if name == "moment" {
			momentVersion = version
			break
		}
	}

	suite.Require().NotEmpty(momentVersion, "Can't find moment version")
	return momentVersion[1:] // Drop the ^ prefix
}

func TestDependencies(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, &depedenciesTestSuite{})
}
