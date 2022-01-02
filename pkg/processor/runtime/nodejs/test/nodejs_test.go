//go:build test_integration && test_local

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

	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	httpsuite.TestSuite
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = "nodejs"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "nodejs", "test")
}

func (suite *TestSuite) TestOutputs() {
	statusOK := http.StatusOK
	statusCreated := http.StatusCreated
	statusInternalError := http.StatusInternalServerError
	logLevelDebug := "debug"
	logLevelWarn := "warn"

	headersContentTypeTextPlain := map[string]string{"content-type": "text/plain"}
	headersContentTypeApplicationJSON := map[string]string{"content-type": "application/json"}
	headersFromResponse := map[string]string{
		"h1":           "v1",
		"h2":           "v2",
		"content-type": "text/plain",
	}
	testPath := "/path/to/nowhere"

	createFunctionOptions := suite.GetDeployOptions("outputter",
		suite.GetFunctionPath("outputter"))

	createFunctionOptions.FunctionConfig.Spec.Handler = "outputter.js:testHandler"

	testRequests := []*httpsuite.Request{
		{
			Name:                       "return string",
			RequestBody:                "return_string",
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       "a string",
			ExpectedResponseStatusCode: &statusOK,
		},
		{
			Name:                       "return string & status",
			RequestBody:                "return_status_and_string",
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       "a string after status",
			ExpectedResponseStatusCode: &statusCreated,
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
			RequestHeaders:             map[string]interface{}{"a": "1", "b": "2"},
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
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
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
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       "returned logs",
			ExpectedResponseStatusCode: &statusCreated,
			ExpectedLogMessages: []string{
				"Warn message",
				"Error message",
			},
		},
		{
			Name:                       "logs - with",
			RequestBody:                "log_with",
			RequestLogLevel:            &logLevelWarn,
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       "returned logs with",
			ExpectedResponseStatusCode: &statusCreated,
			ExpectedLogRecords: []map[string]interface{}{
				{
					"level":   "error",
					"message": "Error message",
					// extra with
					"source": "rabbit",
					"weight": 7.0, // encoding/json return float64 for all numbers
				},
			},
		},
		{
			Name:                       "get",
			RequestMethod:              "GET",
			RequestBody:                "",
			RequestLogLevel:            &logLevelWarn,
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       "GET",
			ExpectedResponseStatusCode: &statusOK,
		},
		{
			Name:                       "fields",
			RequestMethod:              "POST",
			RequestPath:                "/?x=1&y=2",
			RequestBody:                "return_fields",
			RequestLogLevel:            &logLevelWarn,
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       "x=1,y=2",
			ExpectedResponseStatusCode: &statusOK,
		},
		{
			Name:                       "path",
			RequestMethod:              "POST",
			RequestPath:                testPath,
			RequestBody:                "return_path",
			RequestLogLevel:            &logLevelWarn,
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       testPath,
			ExpectedResponseStatusCode: &statusOK,
		},
		{
			Name:                       "error",
			RequestBody:                "return_error",
			RequestLogLevel:            &logLevelWarn,
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseStatusCode: &statusInternalError,
			ExpectedResponseBody:       regexp.MustCompile("some error"),
		},
	}
	suite.DeployFunctionAndRequests(createFunctionOptions, testRequests)
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
