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
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/test/suite"
	"github.com/nuclio/nuclio/test/compare"
)

type TestSuite struct {
	processorsuite.TestSuite
}

func (suite *TestSuite) FunctionBuildRunAndRequest(functionName string,
	functionPath string,
	runtime string,
	ports map[int]int,
	requestPort int,
	requestPath string,
	requestHeaders map[string]string,
	requestBody string,
	expectedResponseHeaders map[string]string,
	expectedResponseBody string,
	expectedResponseStatusCode *int) {

	defaultStatusCode := http.StatusOK
	if expectedResponseStatusCode == nil {
		expectedResponseStatusCode = &defaultStatusCode
	}

	suite.BuildAndRunFunction(functionName, functionPath, runtime, ports, func() bool {
		return suite.SendRequestVerifyResponse(requestPort,
			requestHeaders,
			requestBody,
			expectedResponseHeaders,
			expectedResponseBody,
			expectedResponseStatusCode)
	})
}

func (suite *TestSuite) SendRequestVerifyResponse(requestPort int,
	requestHeaders map[string]string,
	requestBody string,
	expectedResponseHeaders map[string]string,
	expectedResponseBody interface{},
	expectedResponseStatusCode *int) bool {

	// invoke the function
	response, err := http.DefaultClient.Post(fmt.Sprintf("http://localhost:%d", requestPort),
		"text/plain",
		strings.NewReader(requestBody))

	// if we fail to connect, fail
	if err != nil && strings.Contains(err.Error(), "EOF") {
		time.Sleep(500 * time.Millisecond)
		return false
	}

	suite.Require().NoError(err)

	if expectedResponseStatusCode != nil {
		suite.Require().Equal(*expectedResponseStatusCode, response.StatusCode)
	}

	body, err := ioutil.ReadAll(response.Body)
	suite.Require().NoError(err)

	// verify header correctness
	if expectedResponseHeaders != nil {

		// the response may contain more headers. just check that all the expected
		// headers contain the proper values
		for expectedHeaderName, expectedHeaderValue := range expectedResponseHeaders {
			suite.Require().Equal(expectedHeaderValue, response.Header.Get(expectedHeaderName))
		}
	}

	// verify body correctness
	switch typedExpectedResponseBody := expectedResponseBody.(type) {

	// if it's a simple string - just compare
	case string:
		suite.Require().Equal(typedExpectedResponseBody, string(body))

		// if it's a map - assume JSON
	case map[string]interface{}:

		// verify content type is JSON
		suite.Require().Equal("application/json", response.Header.Get("Content-Type"))

		// unmarshall the body
		unmarshalledBody := make(map[string]interface{})
		err := json.Unmarshal(body, &unmarshalledBody)
		suite.Require().NoError(err)

		suite.Require().True(compare.CompareNoOrder(typedExpectedResponseBody, unmarshalledBody))
	}

	return true
}
