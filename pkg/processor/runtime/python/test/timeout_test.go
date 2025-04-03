//go:build test_integration && test_local

/*
Copyright 2023 The Nuclio Authors.

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
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

var (
	requestHeaders = map[string]interface{}{
		"Content-Type": "application/json",
	}
)

type timeoutSuite struct {
	httpsuite.TestSuite
}

type timeoutResponse struct {
	PID int `json:"pid"`
}

func (suite *timeoutSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = "python"
	suite.RuntimeDir = "python"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "python", "test")
}

func (suite *timeoutSuite) TestTimeout() {
	createFunctionOptions := suite.GetDeployOptions("timeout", path.Join(suite.GetTestFunctionsDir(), "python", "timeout"))

	timeout := 500 * time.Millisecond
	createFunctionOptions.FunctionConfig.Spec.EventTimeout = timeout.String()
	createFunctionOptions.FunctionConfig.Spec.Handler = "timeout:handler"
	var oldPID int
	okStatusCode := http.StatusOK
	timeoutStatusCode := http.StatusRequestTimeout
	sleepTime := 5 * time.Second

	suite.DeployFunctionAndRequests(createFunctionOptions, []*httpsuite.Request{
		{
			RequestBody:    suite.genTimeoutRequest(time.Millisecond, true),
			RequestHeaders: requestHeaders,

			ExpectedResponseBody: func(body []byte) {
				response := &timeoutResponse{}
				err := json.Unmarshal(body, response)
				suite.Require().NoErrorf(err, "Can't parse response - %q", string(body))
				oldPID = response.PID
			},
			ExpectedResponseStatusCode: &okStatusCode,
		},
		{
			RequestBody:    suite.genTimeoutRequest(sleepTime, true),
			RequestHeaders: requestHeaders,

			ExpectedResponseStatusCode: &timeoutStatusCode,
		},
		// retry until runtime is back
		{
			RequestBody:    suite.genTimeoutRequest(0, true),
			RequestHeaders: requestHeaders,

			RetryUntilSuccessfulStatusCode: &okStatusCode,
			RetryUntilSuccessfulInterval:   10 * time.Millisecond,
			RetryUntilSuccessfulDuration:   2 * sleepTime,
		},
		{
			RequestBody:    suite.genTimeoutRequest(time.Millisecond, true),
			RequestHeaders: requestHeaders,

			ExpectedResponseBody: func(body []byte) {
				response := &timeoutResponse{}
				err := json.Unmarshal(body, response)
				suite.Require().NoErrorf(err, "Can't parse response - %q", string(body))
				suite.Require().NotEqual(oldPID, response.PID, "Wrapper PID didn't change")
			},
			ExpectedResponseStatusCode: &okStatusCode,
		},
	})
}

func (suite *timeoutSuite) TestTimeoutAsync() {
	createFunctionOptions := suite.GetDeployOptionsAsync("timeout", path.Join(suite.GetTestFunctionsDir(), "python", "timeout"))

	timeout := 500 * time.Millisecond
	createFunctionOptions.FunctionConfig.Spec.EventTimeout = timeout.String()
	createFunctionOptions.FunctionConfig.Spec.Handler = "timeout_async:handler"
	var oldPID int
	okStatusCode := http.StatusOK
	timeoutStatusCode := http.StatusRequestTimeout
	sleepTime := 5 * time.Second

	suite.DeployFunctionAndRequests(createFunctionOptions, []*httpsuite.Request{
		// sending regular request, 200 code expected
		{
			RequestBody:    suite.genTimeoutRequest(time.Millisecond, false),
			RequestHeaders: requestHeaders,

			ExpectedResponseBody: func(body []byte) {
				response := &timeoutResponse{}
				err := json.Unmarshal(body, response)
				suite.Require().NoErrorf(err, "Can't parse response - %q", string(body))
				oldPID = response.PID
			},
			ExpectedResponseStatusCode: &okStatusCode,
		},
		// sending request with non-blocking sleep, timeout expected
		{
			RequestBody:    suite.genTimeoutRequest(sleepTime, false),
			RequestHeaders: requestHeaders,

			ExpectedResponseStatusCode: &timeoutStatusCode,
		},
		// sending another request, 200 code expected (should be processed by another connection)
		{
			RequestBody:    suite.genTimeoutRequest(0, false),
			RequestHeaders: requestHeaders,

			ExpectedResponseStatusCode: &okStatusCode,
		},
		// sending another request with blocking sleep to timeout not only on waiting on response
		// but also on waiting for the connection to be re-established
		{
			RequestBody:    suite.genTimeoutRequest(time.Minute*2, true),
			RequestHeaders: requestHeaders,

			ExpectedResponseStatusCode: &timeoutStatusCode,
		},
		// retry until runtime is back
		{
			RequestBody:    suite.genTimeoutRequest(0, true),
			RequestHeaders: requestHeaders,

			RetryUntilSuccessfulStatusCode: &okStatusCode,
			RetryUntilSuccessfulInterval:   100 * time.Millisecond,
			RetryUntilSuccessfulDuration:   3 * time.Minute,
		},
		// sending another request, 200 code expected, verify that wrapper PID changed
		{
			RequestBody:    suite.genTimeoutRequest(time.Millisecond, true),
			RequestHeaders: requestHeaders,

			ExpectedResponseBody: func(body []byte) {
				response := &timeoutResponse{}
				err := json.Unmarshal(body, response)
				suite.Require().NoErrorf(err, "Can't parse response - %q", string(body))
				suite.Require().NotEqual(oldPID, response.PID, "Wrapper PID didn't change")
			},
			ExpectedResponseStatusCode: &okStatusCode,
		},
	})
}

func (suite *timeoutSuite) genTimeoutRequest(timeout time.Duration, blocking bool) string {
	request := map[string]interface{}{
		"timeout": timeout.String(),
	}
	// only relevant for async processing
	if blocking {
		request["blocking_sleep"] = true
	}
	data, err := json.Marshal(request)
	suite.Require().NoErrorf(err, "Can't encode request - %#v", request)

	return string(data)
}

func TestTimeout(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, &timeoutSuite{})
}
