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

package cloudevents

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"
)

// CloudEventsTestSuite has common functions for cloudevents testing
// Other suites should embed this suite and in SetupSuite set HTTPSuite
type CloudEventsTestSuite struct { // nolint
	HTTPSuite          *httpsuite.TestSuite
	CloudEventsHandler string
}

// TestStructuredCloudEvent tests a structured cloud event
func (suite *CloudEventsTestSuite) TestStructuredCloudEvent() {
	createFunctionOptions := suite.getCreateOptions()
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

	suite.HTTPSuite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		bodyVerifier := func(body []byte) {
			unmarshalledBody := suite.decodeResponse(body)

			require := suite.HTTPSuite.Require()

			require.Equal("valid xml", unmarshalledBody.Body)
			require.Equal(requestPath, unmarshalledBody.Path)
			require.Equal(requestMethod, unmarshalledBody.Method)
			require.Equal("/mycontext", unmarshalledBody.TriggerKind)
			require.Equal("0.1", unmarshalledBody.Version)
			require.Equal("com.example.someevent", unmarshalledBody.Type)
			require.Equal("1.0", unmarshalledBody.TypeVersion)
			require.Equal("A234-1234-1234", string(unmarshalledBody.ID))
			require.Equal(now, unmarshalledBody.Timestamp.Format(time.RFC3339))
			require.Equal(map[string]interface{}{"comExampleExtension": "value"}, unmarshalledBody.Headers)
			require.Equal("text/xml", unmarshalledBody.ContentType)
		}

		testRequest := httpsuite.Request{
			RequestBody:          requestBody,
			RequestHeaders:       map[string]interface{}{"Content-Type": "application/cloudevents+json"},
			RequestPort:          deployResult.Port,
			RequestMethod:        requestMethod,
			RequestPath:          requestPath,
			ExpectedResponseBody: bodyVerifier,
		}

		return suite.HTTPSuite.SendRequestVerifyResponse(&testRequest)
	})
}

// TestBinaryCloudEvent tests a binary cloudsevents
func (suite *CloudEventsTestSuite) TestBinaryCloudEvent() {
	createFunctionOptions := suite.getCreateOptions()
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

	suite.HTTPSuite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		bodyVerifier := func(body []byte) {
			unmarshalledBody := suite.decodeResponse(body)

			var require = suite.HTTPSuite.Require()

			require.Equal(requestBody, unmarshalledBody.Body)
			require.Equal(requestPath, unmarshalledBody.Path)
			require.Equal(requestMethod, unmarshalledBody.Method)
			require.Equal("/mycontext", unmarshalledBody.TriggerKind)
			require.Equal("0.1", unmarshalledBody.Version)
			require.Equal("com.example.someevent", unmarshalledBody.Type)
			require.Equal("1.0", unmarshalledBody.TypeVersion)
			require.Equal("A234-1234-1234", string(unmarshalledBody.ID))
			require.Equal(now, unmarshalledBody.Timestamp.Format(time.RFC3339))
			require.Equal("text/xml", unmarshalledBody.ContentType)
		}

		testRequest := httpsuite.Request{
			RequestBody:          requestBody,
			RequestHeaders:       headers,
			RequestPort:          deployResult.Port,
			RequestMethod:        requestMethod,
			RequestPath:          requestPath,
			ExpectedResponseBody: bodyVerifier,
		}

		return suite.HTTPSuite.SendRequestVerifyResponse(&testRequest)
	})
}

func (suite *CloudEventsTestSuite) getCreateOptions() *platform.CreateFunctionOptions {
	options := suite.HTTPSuite.GetDeployOptions(
		"event-returner",
		path.Join(
			suite.HTTPSuite.GetTestFunctionsDir(),
			"common",
			"event-returner",
			suite.HTTPSuite.GetRuntimeDir(),
		),
	)

	if suite.CloudEventsHandler != "" {
		options.FunctionConfig.Spec.Handler = suite.CloudEventsHandler
	}

	return options
}

func (suite *CloudEventsTestSuite) decodeResponse(body []byte) *httpsuite.EventFields {
	unmarshalledBody := &httpsuite.EventFields{}
	// read the body JSON
	err := json.Unmarshal(body, unmarshalledBody)

	suite.HTTPSuite.Require().NoError(err)

	// Try to decode base64
	decodedBody, err := base64.StdEncoding.DecodeString(unmarshalledBody.Body)
	if err == nil {
		unmarshalledBody.Body = string(decodedBody)
	}

	return unmarshalledBody
}
