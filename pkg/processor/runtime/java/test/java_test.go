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
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/processor/test/offline"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	httpsuite.TestSuite
	OfflineTestSuite offline.TestSuite
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()
	suite.OfflineTestSuite.HTTPSuite = &suite.TestSuite
	suite.OfflineTestSuite.FunctionHandler = "Reverser"

	suite.Runtime = "java"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "java", "test")
}

func (suite *TestSuite) TestOutputs() {
	statusOK := http.StatusOK
	headersContentTypeTextPlain := map[string]string{"content-type": "text/plain"}
	statusCreated := http.StatusCreated
	headersFromResponse := map[string]string{
		"h1":           "v1",
		"h2":           "v2",
		"content-type": "text/plain",
	}
	statusInternalError := http.StatusInternalServerError
	logLevelDebug := "debug"
	logLevelWarn := "warn"
	testPath := "/path/to/nowhere"

	createFunctionOptions := suite.GetDeployOptions("outputter",
		suite.GetFunctionPath("outputter"))
	createFunctionOptions.FunctionConfig.Spec.Handler = "Outputter"

	testRequests := []*httpsuite.Request{
		{
			Name:                       "return string",
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

func (suite *TestSuite) TestCustomOptions() {
	createFunctionOptions := suite.GetDeployOptions("memory",
		path.Join(suite.GetTestFunctionsDir(), "java", "memory"))

	createFunctionOptions.FunctionConfig.Spec.Handler = "nuclio-test-memory-handler.jar:MemoryHandler"
	bodyVerifier := func(body []byte) {
		maxMemory := "Max: 536870912"

		bodyStr := string(body)
		suite.Require().Truef(strings.Contains(bodyStr, maxMemory), "bad response:\n%s", bodyStr)
	}
	suite.DeployFunctionAndRequest(createFunctionOptions, &httpsuite.Request{
		RequestMethod:        "GET",
		ExpectedResponseBody: bodyVerifier,
	})
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
