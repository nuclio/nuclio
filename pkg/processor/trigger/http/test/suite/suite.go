//go:build test_unit || test_integration || test_kube || test_local

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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"
	"github.com/nuclio/nuclio/test/compare"

	"github.com/nuclio/nuclio-sdk-go"
)

// EventFields for events
type EventFields struct {
	ID             nuclio.ID              `json:"id,omitempty"`
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

// Request holds information about test HTTP request and response
type Request struct {
	Name string

	RequestBody     string
	RequestHeaders  map[string]interface{}
	RequestLogLevel *string
	RequestMethod   string
	RequestPath     string
	RequestPort     int

	ExpectedLogMessages           []string
	ExpectedLogRecords            []map[string]interface{}
	ExpectedResponseBody          interface{}
	ExpectedResponseHeaders       map[string]string
	ExpectedResponseHeadersValues map[string][]string
	ExpectedResponseStatusCode    *int

	RetryUntilSuccessfulStatusCode *int
	RetryUntilSuccessfulDuration   time.Duration
	RetryUntilSuccessfulInterval   time.Duration
}

func (r *Request) Enrich(deployResult *platform.CreateFunctionResult) {
	defaultStatusCode := http.StatusOK
	if r.ExpectedResponseStatusCode == nil {
		r.ExpectedResponseStatusCode = &defaultStatusCode
	}

	if r.RequestPort == 0 {
		r.RequestPort = deployResult.Port
	}

	if r.RequestPath == "" {
		r.RequestPath = "/"
	}

	if r.RequestMethod == "" {
		r.RequestMethod = "POST"
	}
}

// TestSuite is an HTTP test suite
type TestSuite struct {
	processorsuite.TestSuite
	httpClient *http.Client
	Ctx        context.Context
}

// SetupTest runs before every test
func (suite *TestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	suite.Ctx = context.Background()
}

// DeployFunctionAndExpectError runs a function, expecting an error
func (suite *TestSuite) DeployFunctionAndExpectError(createFunctionOptions *platform.CreateFunctionOptions, expectedMessage string) {

	// add some more common CreateFunctionOptions
	suite.PopulateDeployOptions(createFunctionOptions)

	_, err := suite.Platform.CreateFunction(suite.Ctx, createFunctionOptions)
	suite.Require().Error(err, expectedMessage)
}

// DeployFunctionAndRequest deploys a function and call it with request
func (suite *TestSuite) DeployFunctionAndRequest(createFunctionOptions *platform.CreateFunctionOptions,
	request *Request) *platform.CreateFunctionResult {
	return suite.DeployFunctionAndRequests(createFunctionOptions, []*Request{request})
}

// DeployFunctionAndRequests deploys a function and call it with multiple requests
func (suite *TestSuite) DeployFunctionAndRequests(createFunctionOptions *platform.CreateFunctionOptions,
	requests []*Request) *platform.CreateFunctionResult {

	return suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult)
		for _, request := range requests {
			request.Enrich(deployResult)
			if !suite.SendRequestVerifyResponse(request) {

				// fail fast
				return false
			}
		}
		return true
	})
}

// SendRequestVerifyResponse sends a request and verifies we got expected response
func (suite *TestSuite) SendRequestVerifyResponse(request *Request) bool {
	var httpResponse *http.Response
	var err error

	// retry
	if request.RetryUntilSuccessfulStatusCode != nil {
		err = common.RetryUntilSuccessful(request.RetryUntilSuccessfulDuration,
			request.RetryUntilSuccessfulInterval,
			func() bool {
				httpResponse, err = suite.sendRequest(request)
				if err != nil {
					return false
				}
				return httpResponse.StatusCode == *request.RetryUntilSuccessfulStatusCode
			})

	} else {
		httpResponse, err = suite.sendRequest(request)
	}

	// if we fail to connect, fail, so callee might retry
	if err != nil && common.MatchStringPatterns([]string{

		// function is not up yet
		"EOF",
		"connection reset by peer",

		// https://github.com/golang/go/issues/19943#issuecomment-355607646
		// tl;dr: we should actively retry on such errors, because Go won't as request might not be idempotent
		"server closed idle connection",
	}, err.Error()) {
		time.Sleep(500 * time.Millisecond)
		return false
	}

	suite.Require().NoError(err, "Failed to send request")

	body, err := ioutil.ReadAll(httpResponse.Body)
	suite.Require().NoError(err)

	if request.ExpectedResponseStatusCode != nil {
		suite.Require().Equal(*request.ExpectedResponseStatusCode,
			httpResponse.StatusCode,
			"Got unexpected status code with request body (%s) and response body (%s)",
			request.RequestBody,
			body)
	}

	// verify header correctness
	// the httpResponse may contain more headers. just check that all the expected

	if request.ExpectedResponseHeaders != nil {

		// headers contain the proper values
		for expectedHeaderName, expectedHeaderValue := range request.ExpectedResponseHeaders {
			suite.Require().Equal(expectedHeaderValue, httpResponse.Header.Get(expectedHeaderName))
		}
	}

	if request.ExpectedResponseHeadersValues != nil {

		// header may contain list of values
		for expectedHeaderName, expectedHeaderValues := range request.ExpectedResponseHeadersValues {
			suite.Require().Equal(expectedHeaderValues, httpResponse.Header.Values(expectedHeaderName))
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

		suite.Require().True(compare.NoOrder(typedExpectedResponseBody, unmarshalledBody))
	case *regexp.Regexp:
		suite.Require().Regexp(typedExpectedResponseBody, string(body))
	case func([]byte):
		typedExpectedResponseBody(body)
	case func([]byte, int):
		typedExpectedResponseBody(body, httpResponse.StatusCode)
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

func (suite *TestSuite) sendRequest(request *Request) (*http.Response, error) {
	suite.Logger.DebugWith("Sending request",
		"requestPort", request.RequestPort,
		"requestPath", request.RequestPath,
		"requestHeaders", request.RequestHeaders,
		"requestBodyLength", len(request.RequestBody),
		"requestLogLevel", request.RequestLogLevel)

	// Send request to proper url
	url := fmt.Sprintf("http://%s:%d%s", suite.GetTestHost(), request.RequestPort, request.RequestPath)

	// create a request
	httpRequest, err := http.NewRequest(request.RequestMethod, url, strings.NewReader(request.RequestBody))
	suite.Require().NoError(err)

	// if there are request headers, add them
	if request.RequestHeaders != nil {
		for requestHeaderName, requestHeaderValue := range request.RequestHeaders {
			httpRequest.Header.Add(requestHeaderName, fmt.Sprintf("%v", requestHeaderValue))
		}
	} else {
		httpRequest.Header.Add("Content-Type", "text/plain")
	}

	// if there is a log level, add the header
	if request.RequestLogLevel != nil {
		httpRequest.Header.Add("X-nuclio-log-level", *request.RequestLogLevel)
	}

	// invoke the function
	return suite.httpClient.Do(httpRequest)
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
