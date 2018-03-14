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
	"os"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	httpsuite.TestSuite
}

func (suite *TestSuite) TestDeployDebugLevel() {
	statusOK := http.StatusOK
	logLevelDebug := "debug"

	deployOptions := suite.GetDeployOptions("logging",
		suite.GetFunctionPath(functionPath(), "common", "logging", "golang"))

	deployOptions.FunctionConfig.Spec.Handler = "main:Logging"
	deployOptions.FunctionConfig.Spec.Runtime = "golang"

	suite.DeployFunction(deployOptions, func(deployResult *platform.DeployResult) bool {
		testRequests := []httpsuite.Request{
			{
				Name: "logging",
				ExpectedResponseStatusCode: &statusOK,
				RequestLogLevel:            &logLevelDebug,
				ExpectedLogRecords: []map[string]interface{}{
					{
						"level":   "debug",
						"message": "Debug message",
					},
					{
						"level":   "info",
						"message": "Incrementing body",
					},
					{
						"level":   "warn",
						"message": "Incrementing body",
					},
					{
						"level":   "error",
						"message": "Incrementing body",
					},
				},
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

func (suite *TestSuite) TestDeployInfoLevel() {
	statusOK := http.StatusOK
	logLevelDebug := "info"

	deployOptions := suite.GetDeployOptions("logging",
		suite.GetFunctionPath(functionPath(), "common", "logging", "golang"))

	deployOptions.FunctionConfig.Spec.Handler = "main:Logging"
	deployOptions.FunctionConfig.Spec.Runtime = "golang"

	suite.DeployFunction(deployOptions, func(deployResult *platform.DeployResult) bool {
		testRequests := []httpsuite.Request{
			{
				Name: "logging",
				ExpectedResponseStatusCode: &statusOK,
				RequestLogLevel:            &logLevelDebug,
				ExpectedLogRecords: []map[string]interface{}{
					{
						"level":   "info",
						"message": "Incrementing body",
					},
					{
						"level":   "warn",
						"message": "Incrementing body",
					},
					{
						"level":   "error",
						"message": "Incrementing body",
					},
				},
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

func functionPath() string {
	return path.Join(os.Getenv("GOPATH"), "src", "github.com", "nuclio", "nuclio", "test", "_functions")
}
