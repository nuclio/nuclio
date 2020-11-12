/*
Copyright 2018 The Nuclio Authors.

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
	"net/http"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/stretchr/testify/suite"
	"github.com/valyala/fasthttp"
)

type HTTPTestSuite struct {
	TestSuite
	triggerName string
}

func (suite *HTTPTestSuite) SetupTest() {
	suite.TestSuite.SetupTest()
	suite.triggerName = "testHTTP"
}

func (suite *HTTPTestSuite) TestCORS() {
	exposeHeaders := "x-nuclio-something, x-nuclio-somethingelse"
	allowHeaders := "Accept, Content-Length, Content-Type, X-nuclio-log-level"
	allowMethods := "OPTIONS, GET, POST, HEAD, PUT"
	preflightMaxAgeSeconds := 10
	origin := "foo.bar"
	createFunctionOptions := suite.getHTTPDeployOptions()
	createFunctionOptions.FunctionConfig.Spec.Triggers[suite.triggerName].Attributes["cors"] = map[string]interface{}{
		"enabled":                true,
		"allowOrigins":           []string{origin},
		"allowHeaders":           strings.Split(allowHeaders, ", "),
		"allowMethods":           strings.Split(allowMethods, ", "),
		"exposeHeaders":          strings.Split(exposeHeaders, ", "),
		"preflightMaxAgeSeconds": preflightMaxAgeSeconds,
	}
	validPreflightResponseStatusCode := fasthttp.StatusOK
	invalidPreflightResponseStatusCode := fasthttp.StatusBadRequest
	suite.DeployFunctionAndRequests(createFunctionOptions,
		[]*Request{

			// Happy flow
			{
				RequestMethod: http.MethodOptions,
				RequestHeaders: map[string]interface{}{
					"Origin":                         origin,
					"Access-Control-Request-Method":  http.MethodPost,
					"Access-Control-Request-Headers": "X-nuclio-log-level",
				},
				ExpectedResponseStatusCode: &validPreflightResponseStatusCode,
				ExpectedResponseHeadersValues: map[string][]string{
					"Access-Control-Allow-Methods":  {allowMethods},
					"Access-Control-Allow-Headers":  {allowHeaders},
					"Access-Control-Allow-Origin":   {origin},
					"Access-Control-Max-Age":        {strconv.Itoa(preflightMaxAgeSeconds)},
				},
			},
			{
				RequestMethod: http.MethodPost,
				RequestHeaders: map[string]interface{}{
					"Origin": origin,
				},
				ExpectedResponseHeadersValues: map[string][]string{
					"Access-Control-Expose-Headers": {exposeHeaders},
				},
			},

			// Disallowed request method
			{
				RequestMethod: http.MethodOptions,
				RequestHeaders: map[string]interface{}{
					"Origin":                        origin,
					"Access-Control-Request-Method": "ABC",
				},
				ExpectedResponseStatusCode: &invalidPreflightResponseStatusCode,
			},

			// Disallowed origin
			{
				RequestMethod: http.MethodOptions,
				RequestHeaders: map[string]interface{}{
					"Origin":                        "dummy-origin",
					"Access-Control-Request-Method": http.MethodPost,
				},
				ExpectedResponseStatusCode: &invalidPreflightResponseStatusCode,
			},
		})
}

func (suite *HTTPTestSuite) TestMaxRequestBodySize() {
	createFunctionOptions := suite.getHTTPDeployOptions()
	maxRequestBodySize := 64
	createFunctionOptions.FunctionConfig.Spec.Triggers[suite.triggerName].
		Attributes["maxRequestBodySize"] = maxRequestBodySize
	statusOK := fasthttp.StatusOK
	statusBadRequest := fasthttp.StatusBadRequest
	suite.DeployFunctionAndRequests(createFunctionOptions,
		[]*Request{
			// Happy flows
			{
				RequestMethod:              "POST",
				RequestBody:                string(make([]byte, maxRequestBodySize-1)),
				ExpectedResponseStatusCode: &statusOK,
			},
			{
				RequestMethod:              "POST",
				RequestBody:                string(make([]byte, maxRequestBodySize)),
				ExpectedResponseStatusCode: &statusOK,
			},

			// Bad flow
			{
				RequestMethod:              "POST",
				RequestBody:                string(make([]byte, maxRequestBodySize+1)),
				ExpectedResponseStatusCode: &statusBadRequest,
			},
		})
}

func (suite *HTTPTestSuite) TestFunctionEvent() {
	createFunctionOptions := suite.getHTTPDeployOptions()
	requestPath := "/../../here"
	suite.DeployFunctionAndRequests(createFunctionOptions, []*Request{
		{
			RequestPath: requestPath,
			RequestHeaders: map[string]interface{}{
				"x-nuclio-invoke-trigger": "http-event-recorder",
			},
			ExpectedResponseBody: func(body []byte) {
				unmarshalledBody := EventFields{}
				err := json.Unmarshal(body, &unmarshalledBody)
				suite.Require().NoError(err)

				// path remain the same
				suite.Require().Equal(requestPath, unmarshalledBody.Path)
			},
		},
	})
}

func (suite *HTTPTestSuite) getHTTPDeployOptions() *platform.CreateFunctionOptions {
	createFunctionOptions := suite.GetDeployOptions("http-trigger-test",
		path.Join(suite.GetTestFunctionsDir(), "common", "event-returner", "python"))
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python"
	createFunctionOptions.FunctionConfig.Spec.Handler = "eventreturner:handler"
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		suite.triggerName: {
			Kind:       "http",
			Attributes: map[string]interface{}{},
		},
	}
	return createFunctionOptions
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(HTTPTestSuite))
}
