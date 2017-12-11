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
)

type TestSuite struct {
	httpsuite.TestSuite
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = "golang"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "golang", "test")
}

func (suite *TestSuite) TestOutputs() {
	// TODO: Have common tests and use here and in Python
	// see https://github.com/nuclio/nuclio/issues/227

	statusOK := http.StatusOK
	statusCreated := http.StatusCreated
	statusInternalError := http.StatusInternalServerError
	logLevelDebug := "debug"
	logLevelWarn := "warn"
	testPath := "/path/to/nowhere"

	headersContentTypeTextPlain := map[string]string{"content-type": "text/plain; charset=utf-8"}
	// headersContentTypeApplicationJSON := map[string]string{"content-type": "application/json"}

	deployOptions := suite.GetDeployOptions("outputter",
		suite.GetFunctionPath("_outputter"))

	suite.DeployFunction(deployOptions, func(deployResult *platform.DeployResult) bool {

		testRequests := []httpsuite.Request{
			{
				Name:                       "string",
				RequestBody:                "return_string",
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseBody:       "a string",
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                       "bytes",
				RequestBody:                "return_bytes",
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseBody:       "bytes",
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                       "panic",
				RequestBody:                "panic",
				ExpectedResponseStatusCode: &statusInternalError,
			},
			{
				Name:           "response object",
				RequestHeaders: map[string]string{"a": "1", "b": "2"},
				RequestBody:    "return_response",
				ExpectedResponseHeaders: map[string]string{
					"a":            "1",
					"b":            "2",
					"h1":           "v1",
					"h2":           "v2",
					"Content-Type": "text/plain; charset=utf-8",
				},
				ExpectedResponseBody:       "response body",
				ExpectedResponseStatusCode: &statusCreated,
			},
			{
				Name:                       "logs - debug",
				RequestBody:                "log",
				RequestLogLevel:            &logLevelDebug,
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseBody:       "returned logs",
				ExpectedResponseStatusCode: &statusOK,
				ExpectedLogMessages: []string{
					"Debug message",
					"Info message",
					"Warn message",
					"Error message",
				},
			},
			{
				Name:                       "logs - warn",
				RequestBody:                "log",
				RequestLogLevel:            &logLevelWarn,
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseBody:       "returned logs",
				ExpectedResponseStatusCode: &statusOK,
				ExpectedLogMessages: []string{
					"Warn message",
					"Error message",
				},
			},
			{
				Name:                 "GET",
				RequestMethod:        "GET",
				ExpectedResponseBody: "GET",
			},
			{
				Name:                       "fields",
				RequestPath:                "/?x=1&y=2",
				RequestBody:                "return_fields",
				RequestLogLevel:            &logLevelWarn,
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseBody:       "x=1,y=2",
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                 "path",
				RequestBody:          "return_path",
				RequestPath:          testPath,
				ExpectedResponseBody: testPath,
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
