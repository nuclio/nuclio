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
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type offlineTestSuite struct { // nolint
	httpsuite.TestSuite
}

func (suite *offlineTestSuite) TestGolang() {
	createFunctionOptions := suite.GetDeployOptions("withvendor",
		path.Join(suite.GetTestFunctionsDir(), "golang", "with-vendor"))

	createFunctionOptions.FunctionConfig.Spec.Build.Offline = true
	createFunctionOptions.FunctionConfig.Spec.Build.NoCache = true
	createFunctionOptions.FunctionConfig.Spec.Runtime = "golang"

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		testRequest := httpsuite.Request{
			RequestMethod:        "GET",
			RequestPort:          deployResult.Port,
			ExpectedResponseBody: "from_vendor",
		}

		if !suite.SendRequestVerifyResponse(&testRequest) {
			return false
		}

		return true
	})
}

func (suite *offlineTestSuite) TestJava() {
	createFunctionOptions := suite.GetDeployOptions("reverser",
		path.Join(suite.GetTestFunctionsDir(), "common", "reverser", "java"))

	createFunctionOptions.FunctionConfig.Spec.Build.Offline = true
	createFunctionOptions.FunctionConfig.Spec.Build.NoCache = true
	createFunctionOptions.FunctionConfig.Spec.Runtime = "java"
	createFunctionOptions.FunctionConfig.Spec.Handler = "Reverser"

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		testRequest := httpsuite.Request{
			RequestBody:          "abcd",
			RequestMethod:        "POST",
			RequestPort:          deployResult.Port,
			ExpectedResponseBody: "dcba",
		}

		if !suite.SendRequestVerifyResponse(&testRequest) {
			return false
		}

		return true
	})
}

func TestOfflineSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(offlineTestSuite))
}
