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

package httpsuite

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"
	"github.com/nuclio/nuclio/test/compare"
)

var (
	defaultContainerTimeout = 5 * time.Second
)

// Request holds information about test HTTP request and response
type Request struct {
	Name string

	RequestBody     string
	RequestHeaders  map[string]string
	RequestLogLevel *string
	RequestMethod   string
	RequestPath     string
	RequestPort     int

	ExpectedLogMessages        []string
	ExpectedLogRecords         []map[string]interface{}
	ExpectedResponseBody       interface{}
	ExpectedResponseHeaders    map[string]string
	ExpectedResponseStatusCode *int
}

// TestSuite is an HTTP test suite
type TestSuite struct {
	processorsuite.TestSuite
	httpClient *http.Client
}

// SetupTest runs before every test
func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.httpClient = &http.Client{
		Timeout: 5 * time.Second,
	}
}

func (suite *TestSuite) DeployFunctionAndRequest(deployOptions *platform.DeployOptions,
	request *Request) *platform.DeployResult {

	defaultStatusCode := http.StatusOK
	if request.ExpectedResponseStatusCode == nil {
		request.ExpectedResponseStatusCode = &defaultStatusCode
	}

	// by default BuildAndRunFunction will map 8080
	if request.RequestPort == 0 {
		request.RequestPort = 8080
	}

	if request.RequestPath == "" {
		request.RequestPath = "/"
	}

	if request.RequestMethod == "" {
		request.RequestMethod = "POST"
	}

	return suite.DeployFunction(deployOptions, func(deployResult *platform.DeployResult) bool {
		suite.WaitForContainer(deployResult.Port)

		// modify request port to that of the deployed
		request.RequestPort = deployResult.Port

		return suite.SendRequestVerifyResponse(request)
	})
}

// SendRequestVerifyResponse sends a request and verifies we got expected response
func (suite *TestSuite) SendRequestVerifyResponse(request *Request) bool {

	suite.Logger.DebugWith("Sending request",
		"requestPort", request.RequestPort,
		"requestPath", request.RequestPath,
		"requestHeaders", request.RequestHeaders,
		"requestBody", request.RequestBody,
		"requestLogLevel", request.RequestLogLevel)

	url := fmt.Sprintf("http://localhost:%d%s", request.RequestPort, request.RequestPath)

	// create a request
	httpRequest, err := http.NewRequest(request.RequestMethod, url, strings.NewReader(request.RequestBody))
	suite.Require().NoError(err)

	// if there are request headers, add them
	if request.RequestHeaders != nil {
		for requestHeaderName, requestHeaderValue := range request.RequestHeaders {
			httpRequest.Header.Add(requestHeaderName, requestHeaderValue)
		}
	} else {
		httpRequest.Header.Add("Content-Type", "text/plain")
	}

	// if there is a log level, add the header
	if request.RequestLogLevel != nil {
		httpRequest.Header.Add("X-nuclio-log-level", *request.RequestLogLevel)
	}

	// invoke the function
	httpResponse, err := suite.httpClient.Do(httpRequest)

	// if we fail to connect, fail
	if err != nil && strings.Contains(err.Error(), "EOF") {
		time.Sleep(500 * time.Millisecond)
		return false
	}

	suite.Require().NoError(err)

	if request.ExpectedResponseStatusCode != nil {
		suite.Require().Equal(*request.ExpectedResponseStatusCode,
			httpResponse.StatusCode,
			"Got unexpected status code with request body (%s)",
			request.RequestBody)
	}

	body, err := ioutil.ReadAll(httpResponse.Body)
	suite.Require().NoError(err)

	// verify header correctness
	if request.ExpectedResponseHeaders != nil {
		// the httpResponse may contain more headers. just check that all the expected
		// headers contain the proper values
		for expectedHeaderName, expectedHeaderValue := range request.ExpectedResponseHeaders {
			suite.Require().Equal(expectedHeaderValue, httpResponse.Header.Get(expectedHeaderName))
		}
	}

	// verify body correctness
	switch typedExpectedResponseBody := request.ExpectedResponseBody.(type) {

	// if it's a simple string - just compare
	case string:
		suite.Require().Equal(typedExpectedResponseBody, string(body))

		// if it's a map - assume JSON
	case map[string]interface{}:

		// verify content type is JSON
		suite.Require().Equal("application/json", httpResponse.Header.Get("Content-Type"))

		// unmarshall the body
		unmarshalledBody := make(map[string]interface{})
		err := json.Unmarshal(body, &unmarshalledBody)
		suite.Require().NoError(err)

		suite.Require().True(compare.CompareNoOrder(typedExpectedResponseBody, unmarshalledBody))
	case *regexp.Regexp:
		suite.Require().Regexp(typedExpectedResponseBody, string(body))
	}

	// if there are logs expected, verify them
	if request.ExpectedLogMessages != nil {
		decodedLogRecords := []map[string]interface{}{}

		// decode the logs in the header
		encodedLogs := httpResponse.Header.Get("X-nuclio-logs")
		err := json.Unmarshal([]byte(encodedLogs), &decodedLogRecords)
		suite.Require().NoError(err)

		receivedLogMessages := []string{}

		// create a list of messages
		for _, decodedLogRecord := range decodedLogRecords {

			// add the message to the list
			receivedLogMessages = append(receivedLogMessages, decodedLogRecord["message"].(string))
		}

		// now compare the expected and received logs
		suite.Require().Equal(request.ExpectedLogMessages, receivedLogMessages)
	}

	if request.ExpectedLogRecords != nil {
		decodedLogRecords := []map[string]interface{}{}

		// decode the logs in the header
		encodedLogs := httpResponse.Header.Get("X-nuclio-logs")
		err := json.Unmarshal([]byte(encodedLogs), &decodedLogRecords)
		suite.Require().NoError(err)
		suite.Require().Equal(len(request.ExpectedLogRecords), len(decodedLogRecords))

		for i, expected := range request.ExpectedLogRecords {
			logRecord := decodedLogRecords[i]
			subLogRecord := suite.subMap(logRecord, expected)
			suite.Require().Equal(expected, subLogRecord)
		}
	}

	return true
}

// subMap returns a subset of source with only the keys in keys
// e.g. subMap({"a": 1, "b": 2, "c": 3}, {"b": 7, "c": 20}) -> {"b": 2, "c": 3}
func (suite *TestSuite) subMap(source, keys map[string]interface{}) map[string]interface{} {
	sub := make(map[string]interface{})
	for key := range keys {
		sub[key] = source[key]
	}

	return sub
}

// WaitForContainer wait for container to be ready on port
func (suite *TestSuite) WaitForContainer(port int) error {
	start := time.Now()
	url := fmt.Sprintf("http://localhost:%d", port)
	var err error

	for time.Since(start) <= defaultContainerTimeout {
		_, err = http.Get(url)
		if err == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}

	return err
}
