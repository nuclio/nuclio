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

	headersContentTypeTextPlain := map[string]string{"content-type": "text/plain; charset=utf-8"}
	// headersContentTypeApplicationJSON := map[string]string{"content-type": "application/json"}

	suite.BuildAndRunFunction("outputter",
		path.Join(suite.GetGolangDir(), "outputter"),
		"golang",
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
					// function returns bytes
					RequestBody:                "return_bytes",
					ExpectedResponseHeaders:    headersContentTypeTextPlain,
					ExpectedResponseBody:       "bytes",
					ExpectedResponseStatusCode: &statusOK,
				},
				{
					// function panics
					RequestBody:                "panic",
					ExpectedResponseStatusCode: &statusInternalError,
				},
				{
					// function returns a response object
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
					// function returns logs - ask for all logs
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
					// function returns logs - ask for all logs equal to or above warn
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
					// Different method
					RequestMethod:        "GET",
					ExpectedResponseBody: "GET",
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

func (suite *TestSuite) GetGolangDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "golang", "test")
}

func TestTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
