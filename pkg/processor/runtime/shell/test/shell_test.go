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
	"net/http"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
)

type TestSuite struct {
	httpsuite.TestSuite
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = "shell"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "shell", "test")
}

func (suite *TestSuite) TestOutputs() {
	statusOK := http.StatusOK
	statusInternalError := http.StatusInternalServerError

	expectedResponseHeaders := map[string]string{
		"content-type": "text/plain; charset=utf-8",
		"header1": "value1",
	}

	deployOptions := suite.GetDeployOptions("outputter",
		suite.GetFunctionPath("outputter"))

	deployOptions.FunctionConfig.Spec.Handler = "outputter.sh:main"
	deployOptions.FunctionConfig.Spec.Env = []v1.EnvVar{
		{Name: "ENV1", Value: "value1"},
		{Name: "ENV2", Value: "value2"},
	}
	deployOptions.FunctionConfig.Spec.RuntimeAttributes = map[string]interface{}{
		"arguments":       "first second",
		"responseHeaders": map[string]interface{}{"header1": "value1"},
	}

	suite.DeployFunction(deployOptions, func(deployResult *platform.DeployResult) bool {
		err := suite.WaitForContainer(deployResult.Port)
		suite.Require().NoError(err, "Can't reach container on port %d", deployResult.Port)

		testRequests := []httpsuite.Request{
			{
				Name:                       "return body",
				RequestBody:                "return_body",
				ExpectedResponseHeaders:    expectedResponseHeaders,
				ExpectedResponseBody:       "return_body\n",
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                       "return environment variables",
				RequestBody:                "return_env",
				ExpectedResponseHeaders:    expectedResponseHeaders,
				ExpectedResponseBody:       "value1-value2\n",
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                       "return error",
				RequestBody:                "return_error",
				ExpectedResponseStatusCode: &statusInternalError,
			},
			{
				Name:                       "return arguments",
				RequestBody:                "return_arguments",
				ExpectedResponseHeaders:    expectedResponseHeaders,
				ExpectedResponseBody:       "first-second\n",
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name: "return overridden arguments",
				RequestHeaders: map[string]string{
					"x-nuclio-arguments": "overridefirst overridesecond",
				},
				RequestBody:                "return_arguments",
				ExpectedResponseHeaders:    expectedResponseHeaders,
				ExpectedResponseBody:       "overridefirst-overridesecond\n",
				ExpectedResponseStatusCode: &statusOK,
			},
		}

		for _, testRequest := range testRequests {
			suite.Logger.DebugWith("Running sub test", "name", testRequest.Name)

			// set defaults
			if testRequest.RequestPort == 0 {
				testRequest.RequestPort = deployResult.Port
			}

			if testRequest.RequestMethod == "" {
				testRequest.RequestMethod = "POST"
			}

			if testRequest.RequestPath == "" {
				testRequest.RequestPath = "/"
			}

			if !suite.SendRequestVerifyResponse(&testRequest) {
				return false
			}
		}

		return true
	})
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
