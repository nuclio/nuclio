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

package offline

import (
	"os"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type offlineTestSuite struct { // nolint
	httpsuite.TestSuite
}

func (suite *offlineTestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	// since we build offline, force docker build command to run with --network none
	err := os.Setenv("NUCLIO_DOCKER_BUILD_NETWORK", "none")
	suite.Require().NoError(err)
}

func (suite *offlineTestSuite) TestGolang() {
	suite.T().Skipf("TODO: Once will be able to pass go mod cache from processor to function plugin")
	createFunctionOptions := suite.GetDeployOptions("withmodules",
		path.Join(suite.GetTestFunctionsDir(), "golang", "with-modules"))

	createFunctionOptions.FunctionConfig.Spec.Build.Offline = true
	createFunctionOptions.FunctionConfig.Spec.Build.NoCache = true
	createFunctionOptions.FunctionConfig.Spec.Runtime = "golang"
	createFunctionOptions.FunctionConfig.Spec.Handler = "WithModules"
	suite.DeployFunctionAndRequest(createFunctionOptions, &httpsuite.Request{
		RequestMethod:        "GET",
		ExpectedResponseBody: "from_go_modules",
	})
}

func (suite *offlineTestSuite) TestJava() {
	createFunctionOptions := suite.GetDeployOptions("reverser",
		path.Join(suite.GetTestFunctionsDir(), "common", "reverser", "java"))

	createFunctionOptions.FunctionConfig.Spec.Build.Offline = true
	createFunctionOptions.FunctionConfig.Spec.Build.NoCache = true
	createFunctionOptions.FunctionConfig.Spec.Runtime = "java"
	createFunctionOptions.FunctionConfig.Spec.Handler = "Reverser"

	suite.DeployFunctionAndRequest(createFunctionOptions, &httpsuite.Request{
		RequestBody:          "abcd",
		RequestMethod:        "POST",
		ExpectedResponseBody: "dcba",
	})
}

func TestOfflineSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(offlineTestSuite))
}
