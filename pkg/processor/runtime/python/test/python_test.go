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
	"encoding/json"
	"net/http"
	"path"
	"regexp"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/callfunction"
	"github.com/nuclio/nuclio/pkg/processor/test/cloudevents"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/nuclio/nuclio-sdk-go"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/suite"
)

// shared with EventReturner
type eventFields struct {
	ID             nuclio.ID              `json:"id,omitempty"`
	TriggerClass   string                 `json:"triggerClass,omitempty"`
	TriggerKind    string                 `json:"eventType,omitempty"`
	ContentType    string                 `json:"contentType,omitempty"`
	Headers        map[string]interface{} `json:"headers,omitempty"`
	Timestamp      time.Time              `json:"timestamp,omitempty"`
	Path           string                 `json:"path,omitempty"`
	URL            string                 `json:"url,omitempty"`
	Method         string                 `json:"method,omitempty"`
	ShardID        int                    `json:"shardID,omitempty"`
	TotalNumShards int                    `json:"totalNumShards,omitempty"`
	Type           string                 `json:"type,omitempty"`
	TypeVersion    string                 `json:"typeVersion,omitempty"`
	Version        string                 `json:"version,omitempty"`
	Body           string                 `json:"body,omitempty"`
}

type testSuite struct {
	httpsuite.TestSuite
	cloudevents.CloudEventsTestSuite
	callfunction.CallFunctionTestSuite
	runtime string
}

func newTestSuite(runtime string) *testSuite {
	return &testSuite{runtime: runtime}
}

func (suite *testSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = suite.runtime
	suite.RuntimeDir = "python"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "python", "test")
	suite.CloudEventsTestSuite.HTTPSuite = &suite.TestSuite
	suite.CloudEventsTestSuite.CloudEventsHandler = "eventreturner:handler"
	suite.CallFunctionTestSuite.HTTPSuite = &suite.TestSuite
}

func (suite *testSuite) TestStress() {

	// Create blastConfiguration using default configurations + changes for python specification
	blastConfiguration := suite.NewBlastConfiguration()

	// Create stress test using suite.BlastHTTP
	suite.BlastHTTP(blastConfiguration)
}

func (suite *testSuite) TestOutputs() {
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

	createFunctionOptions.FunctionConfig.Spec.Handler = "outputter:handler"

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		testRequests := []httpsuite.Request{
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
			{
				Name:                       "binary",
				RequestMethod:              "POST",
				RequestBody:                "return_binary",
				RequestLogLevel:            &logLevelWarn,
				ExpectedResponseBody:       []byte("hello"),
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

func (suite *testSuite) TestCustomEvent() {
	createFunctionOptions := suite.GetDeployOptions("event-returner",
		path.Join(suite.GetTestFunctionsDir(), "common", "event-returner", "python"))

	createFunctionOptions.FunctionConfig.Spec.Handler = "eventreturner:handler"

	requestMethod := "POST"
	requestPath := "/testPath"
	requestHeaders := map[string]interface{}{
		"Testheaderkey1": "testHeaderValue1",
		"Testheaderkey2": "testHeaderValue2",
	}

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		bodyVerifier := func(body []byte) {
			unmarshalledBody := eventFields{}

			// read the body JSON
			err := json.Unmarshal(body, &unmarshalledBody)
			suite.Require().NoError(err)

			suite.Require().Equal("testBody", string(unmarshalledBody.Body))
			suite.Require().Equal(requestPath, unmarshalledBody.Path)
			suite.Require().Equal(requestMethod, unmarshalledBody.Method)
			suite.Require().Equal("http", unmarshalledBody.TriggerKind)

			// compare known headers
			for requestHeaderKey, requestHeaderValue := range requestHeaders {
				suite.Require().Equal(requestHeaderValue, unmarshalledBody.Headers[requestHeaderKey])
			}

			// ID must be a UUID
			_, err = uuid.FromString(string(unmarshalledBody.ID))
			suite.Require().NoError(err)
		}

		testRequest := httpsuite.Request{
			RequestBody:          "testBody",
			RequestHeaders:       requestHeaders,
			RequestPort:          deployResult.Port,
			RequestMethod:        requestMethod,
			RequestPath:          requestPath,
			ExpectedResponseBody: bodyVerifier,
		}

		if !suite.SendRequestVerifyResponse(&testRequest) {
			return false
		}

		return true
	})
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, newTestSuite("python"))
	suite.Run(t, newTestSuite("python:2.7"))
	suite.Run(t, newTestSuite("python:3.6"))
}
