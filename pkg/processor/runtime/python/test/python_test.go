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

	"github.com/nuclio/nuclio/pkg/processor/eventsource/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	httpsuite.TestSuite
}

func (suite *TestSuite) TestOutputs() {
	// suite.T().Skip()

	statusOK := http.StatusOK
	statusCreated := http.StatusCreated
	statusInternalError := http.StatusInternalServerError
	logLevelDebug := "debug"
	logLevelWarn := "warn"

	headersContentTypeTextPlain := map[string]string{"content-type": "text/plain"}
	headersContentTypeApplicationJSON := map[string]string{"content-type": "application/json"}

	suite.BuildAndRunFunction("outputter",
		path.Join(suite.getPythonDir(), "outputter"),
		"python",
		map[int]int{8080: 8080},
		func() bool {

			testRequests := []httpsuite.Request{
				{
					// function returns a string
					RequestBody:                "return_string",
					ExpectedResponseHeaders:    headersContentTypeTextPlain,
					ExpectedResponseBody:       "a string",
					ExpectedResponseStatusCode: &statusOK,
				},
				{
					// function returns a string
					RequestBody:                "return_status_and_string",
					ExpectedResponseHeaders:    headersContentTypeTextPlain,
					ExpectedResponseBody:       "a string after status",
					ExpectedResponseStatusCode: &statusCreated,
				},
				{
					// function returns a dict - should be converted to JSON
					RequestBody:                "return_dict",
					ExpectedResponseHeaders:    headersContentTypeApplicationJSON,
					ExpectedResponseBody:       map[string]interface{}{"a": "dict", "b": "foo"},
					ExpectedResponseStatusCode: &statusOK,
				},
				{
					// function returns a dict - should be converted to JSON
					RequestBody:                "return_status_and_dict",
					ExpectedResponseHeaders:    headersContentTypeApplicationJSON,
					ExpectedResponseBody:       map[string]interface{}{"a": "dict after status", "b": "foo"},
					ExpectedResponseStatusCode: &statusCreated,
				},
				{
					// function returns a "response" object. TODO: check headers
					// map[string]string{"a": "1", "b": "2", "h1": "v1", "h2": "v2", "Content-Type": "text/plain"}
					RequestHeaders:             map[string]string{"a": "1", "b": "2"},
					RequestBody:                "return_response",
					ExpectedResponseHeaders:    headersContentTypeTextPlain,
					ExpectedResponseBody:       "response body",
					ExpectedResponseStatusCode: &statusCreated,
				},
				{
					// function raises an exception. we want to make sure it
					// continues functioning afterwards
					RequestBody:                "something invalid",
					ExpectedResponseHeaders:    headersContentTypeTextPlain,
					ExpectedResponseStatusCode: &statusInternalError,
				},
				{
					// function returns logs - ask for all logs
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
					// function returns logs - ask for all logs equal to or above warn
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
					// function should return the method we're posting to it
					RequestMethod:              "GET",
					RequestBody:                "",
					RequestLogLevel:            &logLevelWarn,
					ExpectedResponseHeaders:    headersContentTypeTextPlain,
					ExpectedResponseBody:       "GET",
					ExpectedResponseStatusCode: &statusOK,
				},
			}

			for _, testRequest := range testRequests {

				// set defaults
				if testRequest.RequestPort == 0 {
					testRequest.RequestPort = 8080
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

func (suite *TestSuite) getPythonDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "python", "test")
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
