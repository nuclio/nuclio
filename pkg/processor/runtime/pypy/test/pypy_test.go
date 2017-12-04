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
	"regexp"
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

	suite.Runtime = "pypy"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "pypy", "test")
}

func (suite *TestSuite) TestOutputs() {
	statusOK := http.StatusOK
	statusCreated := http.StatusCreated
	statusInternalError := http.StatusInternalServerError
	logLevelDebug := "debug"
	logLevelWarn := "warn"

	headersContentTypeTextPlain := map[string]string{"content-type": "text/plain"}
	headersContentTypeTextPlainUTF8 := map[string]string{"content-type": "text/plain; charset=utf-8"}
	headersContentTypeApplicationJSON := map[string]string{"content-type": "application/json"}
	headersFromResponse := map[string]string{
		"h1": "v1",
		"h2": "v2",
	}
	testPath := "/path/to/nowhere"

	deployOptions := suite.GetDeployOptions("outputter",
		suite.GetFunctionPath("outputter"))

	deployOptions.FunctionConfig.Spec.Handler = "outputter:handler"

	suite.DeployFunction(deployOptions, func(deployResult *platform.DeployResult) bool {

		err := suite.WaitForContainer(deployResult.Port)
		suite.Require().NoError(err, "Can't reach container on port %d", deployResult.Port)

		testRequests := []httpsuite.Request{
			{
				Name:                       "return string",
				RequestBody:                "return_string",
				ExpectedResponseHeaders:    headersContentTypeTextPlainUTF8,
				ExpectedResponseBody:       "a string",
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                       "return string & status",
				RequestBody:                "return_status_and_string",
				ExpectedResponseHeaders:    headersContentTypeTextPlainUTF8,
				ExpectedResponseBody:       "a string after status",
				ExpectedResponseStatusCode: &statusCreated,
			},
			{
				Name:                       "return dict",
				RequestBody:                "return_dict",
				ExpectedResponseHeaders:    headersContentTypeApplicationJSON,
				ExpectedResponseBody:       map[string]interface{}{"a": "dict", "b": "foo"},
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                       "return dict & status",
				RequestBody:                "return_status_and_dict",
				ExpectedResponseHeaders:    headersContentTypeApplicationJSON,
				ExpectedResponseBody:       map[string]interface{}{"a": "dict after status", "b": "foo"},
				ExpectedResponseStatusCode: &statusCreated,
			},
			{
				Name:                       "return response",
				RequestHeaders:             map[string]string{"a": "1", "b": "2"},
				RequestBody:                "return_response",
				ExpectedResponseHeaders:    headersFromResponse,
				ExpectedResponseBody:       "response body",
				ExpectedResponseStatusCode: &statusCreated,
			},
			{
				// function raises an exception. we want to make sure it
				// continues functioning afterwards
				Name:                       "raise exception",
				RequestBody:                "something invalid",
				ExpectedResponseHeaders:    headersContentTypeTextPlain,
				ExpectedResponseStatusCode: &statusInternalError,
			},
			{
				Name:                       "logs - debug",
				RequestBody:                "log",
				RequestLogLevel:            &logLevelDebug,
				ExpectedResponseHeaders:    headersContentTypeTextPlainUTF8,
				ExpectedResponseBody:       "returned logs",
				ExpectedResponseStatusCode: &statusCreated,
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
				ExpectedResponseHeaders:    headersContentTypeTextPlainUTF8,
				ExpectedResponseBody:       "returned logs",
				ExpectedResponseStatusCode: &statusCreated,
				ExpectedLogMessages: []string{
					"Warn message",
					"Error message",
				},
			},
			{
				Name:                       "get",
				RequestMethod:              "GET",
				RequestBody:                "",
				RequestLogLevel:            &logLevelWarn,
				ExpectedResponseHeaders:    headersContentTypeTextPlainUTF8,
				ExpectedResponseBody:       "GET",
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                       "fields",
				RequestMethod:              "POST",
				RequestPath:                "/?x=1&y=2",
				RequestBody:                "return_fields",
				RequestLogLevel:            &logLevelWarn,
				ExpectedResponseHeaders:    headersContentTypeTextPlainUTF8,
				ExpectedResponseBody:       "x=1,y=2",
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				Name:                       "path",
				RequestMethod:              "POST",
				RequestPath:                testPath,
				RequestBody:                "return_path",
				RequestLogLevel:            &logLevelWarn,
				ExpectedResponseHeaders:    headersContentTypeTextPlainUTF8,
				ExpectedResponseBody:       testPath,
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				// function should error
				RequestBody:                "return_error",
				RequestLogLevel:            &logLevelWarn,
				ExpectedResponseStatusCode: &statusInternalError,
				ExpectedResponseBody:       regexp.MustCompile("some error"),
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
