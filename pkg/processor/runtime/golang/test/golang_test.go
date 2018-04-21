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
	"fmt"
	"net/http"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"
	"github.com/satori/go.uuid"

	"github.com/stretchr/testify/suite"
)

// shared with EventReturner
type eventFields struct {
	ID             nuclio.ID              `json:"id,omitempty"`
	TriggerClass   string                 `json:"triggerClass,omitempty"`
	TriggerKind    string                 `json:"eventType,omitempty"`
	ContentType    string                 `json:"ContentType,omitempty"`
	Headers        map[string]interface{} `json:"Headers,omitempty"`
	Timestamp      time.Time              `json:"Timestamp,omitempty"`
	Path           string                 `json:"path,omitempty"`
	URL            string                 `json:"url,omitempty"`
	Method         string                 `json:"method,omitempty"`
	ShardID        int                    `json:"shardID,omitempty"`
	TotalNumShards int                    `json:"totalNumShards,omitempty"`
	Type           string                 `json:"type,omitempty"`
	TypeVersion    string                 `json:"typeVersion,omitempty"`
	Version        string                 `json:"version,omitempty"`
	Body           []byte                 `json:"body,omitempty"`
}

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

func (suite *TestSuite) TestStructuredCloudEvent() {
	createFunctionOptions := suite.GetDeployOptions("event-returner",
		path.Join(suite.GetTestFunctionsDir(), "common", "event-returner", "golang"))

	now := time.Now().UTC().Format(time.RFC3339)

	requestMethod := "POST"
	requestPath := "/testPath"
	requestBody := fmt.Sprintf(`{
    "cloudEventsVersion": "0.1",
    "eventType": "com.example.someevent",
    "eventTypeVersion": "1.0",
    "source": "/mycontext",
    "eventID": "A234-1234-1234",
    "eventTime": "%s",
    "extensions": {
      "comExampleExtension" : "value"
    },
    "contentType": "text/xml",
    "data": "valid xml"
}`, now)

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		bodyVerifier := func(body []byte) {
			unmarshalledBody := eventFields{}

			// read the body JSON
			err := json.Unmarshal(body, &unmarshalledBody)
			suite.Require().NoError(err)

			suite.Require().Equal("valid xml", string(unmarshalledBody.Body))
			suite.Require().Equal(requestPath, unmarshalledBody.Path)
			suite.Require().Equal(requestMethod, unmarshalledBody.Method)
			suite.Require().Equal("/mycontext", unmarshalledBody.TriggerKind)
			suite.Require().Equal("0.1", unmarshalledBody.Version)
			suite.Require().Equal("com.example.someevent", unmarshalledBody.Type)
			suite.Require().Equal("1.0", unmarshalledBody.TypeVersion)
			suite.Require().Equal("A234-1234-1234", string(unmarshalledBody.ID))
			suite.Require().Equal(now, unmarshalledBody.Timestamp.Format(time.RFC3339))
			suite.Require().Equal(map[string]interface{}{"comExampleExtension": "value"}, unmarshalledBody.Headers)
			suite.Require().Equal("text/xml", unmarshalledBody.ContentType)
		}

		testRequest := httpsuite.Request{
			RequestBody:          requestBody,
			RequestHeaders:       map[string]interface{}{"Content-Type": "application/cloudevents+json"},
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

func (suite *TestSuite) TestBinaryCloudEvent() {
	createFunctionOptions := suite.GetDeployOptions("event-returner",
		path.Join(suite.GetTestFunctionsDir(), "common", "event-returner", "golang"))

	now := time.Now().UTC().Format(time.RFC3339)

	headers := map[string]interface{}{
		"Content-Type":          "text/xml",
		"CE-EventID":            "A234-1234-1234",
		"CE-Source":             "/mycontext",
		"CE-EventTime":          now,
		"CE-EventType":          "com.example.someevent",
		"CE-EventTypeVersion":   "1.0",
		"CE-CloudEventsVersion": "0.1",
	}

	requestMethod := "POST"
	requestPath := "/testPath"
	requestBody := "valid xml"

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		bodyVerifier := func(body []byte) {
			unmarshalledBody := eventFields{}

			// read the body JSON
			err := json.Unmarshal(body, &unmarshalledBody)
			suite.Require().NoError(err)

			suite.Require().Equal(requestBody, string(unmarshalledBody.Body))
			suite.Require().Equal(requestPath, unmarshalledBody.Path)
			suite.Require().Equal(requestMethod, unmarshalledBody.Method)
			suite.Require().Equal("/mycontext", unmarshalledBody.TriggerKind)
			suite.Require().Equal("0.1", unmarshalledBody.Version)
			suite.Require().Equal("com.example.someevent", unmarshalledBody.Type)
			suite.Require().Equal("1.0", unmarshalledBody.TypeVersion)
			suite.Require().Equal("A234-1234-1234", string(unmarshalledBody.ID))
			suite.Require().Equal(now, unmarshalledBody.Timestamp.Format(time.RFC3339))
			suite.Require().Equal("text/xml", unmarshalledBody.ContentType)
		}

		testRequest := httpsuite.Request{
			RequestBody:          requestBody,
			RequestHeaders:       headers,
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
