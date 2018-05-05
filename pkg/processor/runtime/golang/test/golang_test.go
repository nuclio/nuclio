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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/cloudevents"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	httpsuite.TestSuite
	cloudevents.CloudEventsTestSuite
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = "golang"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "golang", "test")
	suite.CloudEventsTestSuite.HTTPSuite = &suite.TestSuite
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
	createFunctionOptions := suite.GetDeployOptions("outputter",
		suite.GetFunctionPath("_outputter"))

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
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
				RequestHeaders: map[string]interface{}{"a": "1", "b": "2"},
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

func (suite *TestSuite) TestCustomEvent() {
	createFunctionOptions := suite.GetDeployOptions("event-returner",
		path.Join(suite.GetTestFunctionsDir(), "common", "event-returner", "golang"))

	requestMethod := "POST"
	requestPath := "/testPath"
	requestHeaders := map[string]interface{}{
		"Testheaderkey1": "testHeaderValue1",
		"Testheaderkey2": "testHeaderValue2",
	}

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		bodyVerifier := func(body []byte) {
			unmarshalledBody := httpsuite.EventFields{}

			// read the body JSON
			err := json.Unmarshal(body, &unmarshalledBody)
			suite.Require().NoError(err, "Can't decode JSON response")

			decodedBody, err := base64.StdEncoding.DecodeString(unmarshalledBody.Body)
			suite.Require().NoError(err, "Can't decode body as base64")

			suite.Require().Equal("testBody", string(decodedBody))
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

func (suite *TestSuite) TestStress() {

	// Create blastConfiguration using default configurations + changes for golang specification
	blastConfiguration := suite.NewBlastConfiguration()
	blastConfiguration.FunctionPath = "_outputter"

	// Create stress test using suite.BlastHTTP
	suite.BlastHTTP(blastConfiguration)
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
