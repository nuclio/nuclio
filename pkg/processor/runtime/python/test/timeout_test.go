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
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type timeoutSuite struct {
	httpsuite.TestSuite
	requestBody string
}

type timeoutResponse struct {
	PID int `json:"pid"`
}

func (suite *timeoutSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = "python:3.6"
	suite.RuntimeDir = "python"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "python", "test")
}

func (suite *timeoutSuite) TestTimeout() {
	createFunctionOptions := suite.GetDeployOptions("timeout",
		path.Join(suite.GetTestFunctionsDir(), "python", "timeout"))
	createFunctionOptions.FunctionConfig.Spec.EventTimeout = 10 * time.Millisecond

	createFunctionOptions.FunctionConfig.Spec.Handler = "timeout:handler"
	var oldPID int

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		bodyVerifier := func(body []byte) {
			response := &timeoutResponse{}
			err := json.Unmarshal(body, response)
			suite.Require().NoErrorf(err, "Can't parse response - %q", string(body))
			oldPID = response.PID
		}

		testRequest := httpsuite.Request{
			RequestBody:          suite.genTimeoutRequest(time.Millisecond),
			RequestPort:          deployResult.Port,
			RequestMethod:        "POST",
			ExpectedResponseBody: bodyVerifier,
		}

		if !suite.SendRequestVerifyResponse(&testRequest) {
			return false
		}

		expectedResponseCode := http.StatusRequestTimeout
		testRequest = httpsuite.Request{
			RequestBody:                suite.genTimeoutRequest(time.Second),
			RequestPort:                deployResult.Port,
			RequestMethod:              "POST",
			ExpectedResponseStatusCode: &expectedResponseCode,
		}

		if !suite.SendRequestVerifyResponse(&testRequest) {
			return false
		}

		// TODO: Poll processor for trigger status?
		time.Sleep(time.Second) // Give runtime time to restart

		bodyVerifier = func(body []byte) {
			response := &timeoutResponse{}
			err := json.Unmarshal(body, response)
			suite.Require().NoErrorf(err, "Can't parse response - %q", string(body))
			suite.Require().NotEqual(oldPID, response.PID, "Wrapper PID didn't change")
		}

		// Check that runtime works after restart and we have another process
		testRequest = httpsuite.Request{
			RequestBody:          suite.genTimeoutRequest(time.Millisecond),
			RequestPort:          deployResult.Port,
			RequestMethod:        "POST",
			ExpectedResponseBody: bodyVerifier,
		}

		if !suite.SendRequestVerifyResponse(&testRequest) {
			return false
		}

		return true
	})
}

func (suite *timeoutSuite) genTimeoutRequest(timeout time.Duration) string {
	request := map[string]interface{}{
		"timeout": timeout.Seconds(),
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
