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
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/callfunction/golang"
	"github.com/nuclio/nuclio/pkg/processor/test/cloudevents"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
)

type TestSuite struct {
	httpsuite.TestSuite
	CloudEventsTestSuite  cloudevents.TestSuite
	CallFunctionTestSuite callfunction.TestSuite

	// TODO: enable once we are being able to pass go mod cache from processor to function plugin
	//OfflineTestSuite      offline.TestSuite
}

func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = "golang"
	suite.RuntimeDir = "golang"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "golang", "test")
	suite.CloudEventsTestSuite.HTTPSuite = &suite.TestSuite
	suite.CallFunctionTestSuite.HTTPSuite = &suite.TestSuite

	// TODO: see comment below above TestSuite
	//suite.OfflineTestSuite.HTTPSuite = &suite.TestSuite
	//suite.OfflineTestSuite.FunctionHandler = "reverser:Reverse"
}

func (suite *TestSuite) TestOutputs() {
	statusOK := http.StatusOK
	badRequest := http.StatusBadRequest
	statusCreated := http.StatusCreated
	statusInternalError := http.StatusInternalServerError
	logLevelDebug := "debug"
	logLevelWarn := "warn"
	testPath := "/path/to/nowhere"

	headersContentTypeTextPlain := map[string]string{"content-type": "text/plain; charset=utf-8"}

	// headersContentTypeApplicationJSON := map[string]string{"content-type": "application/json"}
	createFunctionOptions := suite.GetDeployOptions("outputter",
		suite.GetFunctionPath("_outputter"))

	testRequests := []*httpsuite.Request{
		{
			Name:                       "error-check",
			RequestBody:                "return_body_error",
			ExpectedResponseHeaders:    headersContentTypeTextPlain,
			ExpectedResponseBody:       "error string body",
			ExpectedResponseStatusCode: &badRequest,
		},
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
	suite.DeployFunctionAndRequests(createFunctionOptions, testRequests)
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
	suite.DeployFunctionAndRequest(createFunctionOptions, &httpsuite.Request{
		RequestBody:          "testBody",
		RequestHeaders:       requestHeaders,
		RequestMethod:        requestMethod,
		RequestPath:          requestPath,
		ExpectedResponseBody: bodyVerifier,
	})
}

func (suite *TestSuite) TestFileStream() {
	createFunctionOptions := suite.GetDeployOptions("file-streamer",
		path.Join(suite.GetTestFunctionsDir(), "common", "file-streamer", "golang"))

	tempDir := suite.CreateTempDir()
	defer os.RemoveAll(tempDir)
	createFunctionOptions.FunctionConfig.Spec.Volumes = []functionconfig.Volume{
		{
			Volume: v1.Volume{
				Name: "tmp",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: tempDir,
					},
				},
			},
			VolumeMount: v1.VolumeMount{
				Name:      "tmp",
				MountPath: tempDir,
			},
		},
	}

	for _, testRequest := range []struct {
		responseSize    int
		repeat          int
		requestBody     string
		deleteAfterSend bool
	}{
		{
			responseSize:    4 * 1024,
			repeat:          20,
			requestBody:     common.GenerateRandomString(256, common.LettersAndNumbers),
			deleteAfterSend: false,
		},
		{
			responseSize:    10 * 1024 * 1024,
			deleteAfterSend: true,
		},
	} {
		// generate a random file
		tempFile, err := ioutil.TempFile(tempDir, "")
		suite.Require().NoError(err)

		responseBody := common.GenerateRandomString(testRequest.responseSize, common.LettersAndNumbers)

		// write random stuff to the file which we'll expect as a response
		_, err = tempFile.Write([]byte(responseBody))
		suite.Require().NoError(err)

		httpRequest := httpsuite.Request{
			RequestBody:          testRequest.requestBody,
			RequestMethod:        "POST",
			RequestPath:          tempFile.Name(),
			ExpectedResponseBody: responseBody,
			ExpectedResponseHeaders: map[string]string{
				"X-request-body": testRequest.requestBody,
			},
		}

		if testRequest.deleteAfterSend {
			httpRequest.RequestPath += "?delete_after_send=true"
		}

		suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
			httpRequest.Enrich(deployResult)

			for repeatIdx := 0; repeatIdx < testRequest.repeat+1; repeatIdx++ {
				if !suite.SendRequestVerifyResponse(&httpRequest) {
					return false
				}
			}

			return true
		})

		if testRequest.deleteAfterSend {
			// expect file to be removed
			_, err = os.Stat(tempFile.Name())
			suite.Require().Error(err)
		} else {
			err = os.Remove(tempFile.Name())
			suite.Require().NoError(err)
		}
	}
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
