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
	"regexp"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/callfunction/python"
	"github.com/nuclio/nuclio/pkg/processor/test/cloudevents"
	httptrigger "github.com/nuclio/nuclio/pkg/processor/trigger/http"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	httpsuite.TestSuite
	cloudevents.CloudEventsTestSuite
	callfunction.CallFunctionTestSuite
	runtime string
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = suite.runtime
	suite.RuntimeDir = "python"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "python", "test")
	suite.CloudEventsTestSuite.HTTPSuite = &suite.TestSuite
	suite.CloudEventsTestSuite.CloudEventsHandler = "eventreturner:handler"
	suite.CallFunctionTestSuite.HTTPSuite = &suite.TestSuite
}

func (suite *TestSuite) TestStress() {

	// Create blastConfiguration using default configurations + changes for python specification
	blastConfiguration := suite.NewBlastConfiguration()

	// Create stress test using suite.BlastHTTP
	suite.BlastHTTP(blastConfiguration)
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

	createFunctionOptions.FunctionConfig.Spec.Handler = "outputter:handler"

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
	suite.DeployFunctionAndRequests(createFunctionOptions, testRequests)
}

func (suite *TestSuite) TestCustomEvent() {
	createFunctionOptions := suite.GetDeployOptions("event-returner",
		path.Join(suite.GetTestFunctionsDir(), "common", "event-returner", "python"))

	createFunctionOptions.FunctionConfig.Spec.Handler = "eventreturner:handler"

	requestMethod := "POST"
	requestPath := "/testPath"
	requestHeaders := map[string]interface{}{
		"Testheaderkey1": "testHeaderValue1",
		"Testheaderkey2": "testHeaderValue2",
	}

	bodyVerifier := func(body []byte) {
		unmarshalledBody := httpsuite.EventFields{}

		// read the body JSON
		err := json.Unmarshal(body, &unmarshalledBody)
		suite.Require().NoError(err)

		suite.Require().Equal("testBody", unmarshalledBody.Body)
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
	suite.DeployFunctionAndRequest(createFunctionOptions, &httpsuite.Request{
		RequestBody:          "testBody",
		RequestHeaders:       requestHeaders,
		RequestMethod:        requestMethod,
		RequestPath:          requestPath,
		ExpectedResponseBody: bodyVerifier,
	})
}

func (suite *TestSuite) TestContextInitError() {
	createFunctionOptions := suite.GetDeployOptions("context-init-fail",
		path.Join(suite.GetTestFunctionsDir(), "common", "context-init-fail", "python"))

	createFunctionOptions.FunctionConfig.Spec.Handler = "contextinitfail:handler"
	createFunctionOptions.FunctionConfig.Spec.ReadinessTimeoutSeconds = 10

	suite.DeployFunctionExpectError(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		return true
	})
}

func (suite *TestSuite) TestModifiedRequestBodySize() {

	// TODO: make test more generic and run cross runtimes
	maxRequestBodySizes := []int{
		httptrigger.DefaultMaxRequestBodySize / 2,
		httptrigger.DefaultMaxRequestBodySize,
		2 * httptrigger.DefaultMaxRequestBodySize,
		10 * httptrigger.DefaultMaxRequestBodySize,
	}
	for index, maxRequestBodySize := range maxRequestBodySizes {
		createFunctionOptions := suite.GetDeployOptions(fmt.Sprintf("custom-allowed-body-size-%d", index),
			path.Join(suite.GetTestFunctionsDir(), "common", "empty", "python"))
		createFunctionOptions.FunctionConfig.Spec.Handler = "empty:handler"
		createFunctionOptions.FunctionConfig.Spec.Triggers["http"] = functionconfig.Trigger{
			Kind: "http",
			Attributes: map[string]interface{}{
				"maxRequestBodySize": maxRequestBodySize,
			},
		}
		suite.DeployFunctionAndRequest(createFunctionOptions, &httpsuite.Request{
			RequestBody: string(make([]byte, maxRequestBodySize)),
		})
	}
}

func (suite *TestSuite) TestNonUTF8Headers() {
	createFunctionOptions := suite.GetDeployOptions("non-utf8-headers",
		path.Join(suite.GetTestFunctionsDir(), "common", "empty", "python"))
	createFunctionOptions.FunctionConfig.Spec.Handler = "empty:handler"

	nonUTF8String := string([]byte{192, 175})
	internalServerErrorStatus := http.StatusInternalServerError
	suite.DeployFunctionAndRequests(createFunctionOptions, []*httpsuite.Request{
		{
			RequestMethod: http.MethodPost,
			RequestBody:   nonUTF8String,
		},
		{

			// failed, non utf8 headers can not be parsed
			RequestBody:   "testBody",
			RequestMethod: http.MethodPost,
			RequestHeaders: map[string]interface{}{
				"nonUTFHeader": nonUTF8String,
			},
			ExpectedResponseStatusCode: &internalServerErrorStatus,
		},
		{

			// everything is back to normal
			RequestMethod: http.MethodGet,
		},
	})
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	for _, runtime := range []string{
		"python",
		"python:2.7",
		"python:3.6",
	} {
		testSuite := new(TestSuite)
		testSuite.runtime = runtime
		suite.Run(t, testSuite)
	}
}
