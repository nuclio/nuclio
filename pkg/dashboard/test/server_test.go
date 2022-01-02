//go:build test_unit

//
// cannot reside with server.go because of dependency cycle by "github.com/nuclio/nuclio/pkg/dashboard/resource"
//

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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/dashboard/auth"
	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
	_ "github.com/nuclio/nuclio/pkg/dashboard/resource"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	mockplatform "github.com/nuclio/nuclio/pkg/platform/mock"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/google/go-cmp/cmp"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/zap"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
)

type dashboardTestSuite struct {
	suite.Suite
	logger          logger.Logger
	dashboardServer *dashboard.Server
	httpServer      *httptest.Server
	mockPlatform    *mockplatform.Platform
	ctx             context.Context
}

func (suite *dashboardTestSuite) SetupTest() {
	var err error
	trueValue := true

	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.ctx = context.Background()
	suite.mockPlatform = &mockplatform.Platform{}

	templateRepository, err := functiontemplates.NewRepository(suite.logger, []functiontemplates.FunctionTemplateFetcher{})
	suite.Require().NoError(err)

	// create a mock platform
	suite.dashboardServer, err = dashboard.NewServer(suite.logger,
		suite.mockPlatform.GetContainerBuilderKind(),
		"",
		"",
		"",
		suite.mockPlatform,
		true,
		&platformconfig.WebServer{Enabled: &trueValue},
		nil,
		nil,
		"",
		true,
		templateRepository,
		&platformconfig.Config{
			Kube: platformconfig.PlatformKubeConfig{
				DefaultServiceType:             v1.ServiceTypeNodePort,
				DefaultHTTPIngressHostTemplate: "{{ .FunctionName }}.{{ .ProjectName }}.{{ .Namespace }}.test.com",
			},
		},
		"",
		"",
		"",
		&auth.Config{})

	if err != nil {
		panic("Failed to create server")
	}

	// create an http server from the dashboard server
	suite.httpServer = httptest.NewServer(suite.dashboardServer.Router)
}

func (suite *dashboardTestSuite) TeardownTest() {
	suite.httpServer.Close()
}

func (suite *dashboardTestSuite) sendRequest(method string,
	path string,
	requestHeaders map[string]string,
	requestBody io.Reader,
	expectedStatusCode *int,
	encodedExpectedResponse interface{}) (*http.Response, map[string]interface{}) {

	request, err := http.NewRequest(method, suite.httpServer.URL+path, requestBody)
	suite.Require().NoError(err)

	for headerKey, headerValue := range requestHeaders {
		request.Header.Set(headerKey, headerValue)
	}

	response, err := http.DefaultClient.Do(request)
	suite.Require().NoError(err)

	encodedResponseBody, err := ioutil.ReadAll(response.Body)
	suite.Require().NoError(err)

	defer response.Body.Close() // nolint: errcheck

	suite.logger.DebugWith("Got response",
		"status", response.StatusCode,
		"response", string(encodedResponseBody))

	// check if status code was passed
	if expectedStatusCode != nil {
		suite.Require().Equal(*expectedStatusCode, response.StatusCode)
	}

	// if there's an expected status code, verify it
	decodedResponseBody := map[string]interface{}{}

	if encodedExpectedResponse != nil {

		err := json.Unmarshal(encodedResponseBody, &decodedResponseBody)
		suite.Require().NoError(err)

		suite.logger.DebugWith("Comparing expected", "expected", encodedExpectedResponse)

		switch typedEncodedExpectedResponse := encodedExpectedResponse.(type) {
		case string:
			decodedExpectedResponseBody := map[string]interface{}{}

			err := json.Unmarshal([]byte(typedEncodedExpectedResponse), &decodedExpectedResponseBody)
			suite.Require().NoError(err)
			suite.Require().Empty(cmp.Diff(decodedExpectedResponseBody, decodedResponseBody))

		case func(response map[string]interface{}) bool:
			suite.Require().True(typedEncodedExpectedResponse(decodedResponseBody))

		default:
			panic("Unsupported expected response verifier")
		}
	}

	return response, decodedResponseBody
}

//
// Function
//

type functionTestSuite struct {
	dashboardTestSuite
}

func (suite *functionTestSuite) TestGetDetailSuccessful() {
	replicas := 10
	returnedFunction := platform.AbstractFunction{}
	returnedFunction.Config.Meta.Name = "f1"
	returnedFunction.Config.Meta.Namespace = "f1-namespace"
	returnedFunction.Config.Spec.Replicas = &replicas

	// verify
	verifyGetFunctions := func(getFunctionsOptions *platform.GetFunctionsOptions) bool {
		suite.Require().Equal("f1", getFunctionsOptions.Name)
		suite.Require().Equal("f1-namespace", getFunctionsOptions.Namespace)

		return true
	}

	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.MatchedBy(verifyGetFunctions)).
		Return([]platform.Function{&returnedFunction}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-function-namespace": "f1-namespace",
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
	"metadata": {
		"name": "f1",
		"namespace": "f1-namespace"
	},
	"spec": {
		"resources": {},
		"build": {},
		"platform": {},
		"replicas": 10,
		"eventTimeout": ""
	},
	"status": {}
}`

	suite.sendRequest("GET",
		"/api/functions/f1",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) TestGetDetailNoNamespace() {
	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Namespace must exist"})
	suite.sendRequest("GET",
		"/api/functions/f1",
		nil,
		nil,
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) TestGetListSuccessful() {
	returnedFunction1 := platform.AbstractFunction{}
	returnedFunction1.Config.Meta.Name = "f1"
	returnedFunction1.Config.Meta.Namespace = "f-namespace"
	returnedFunction1.Config.Spec.Runtime = "r1"

	returnedFunction2 := platform.AbstractFunction{}
	returnedFunction2.Config.Meta.Name = "f2"
	returnedFunction2.Config.Meta.Namespace = "f-namespace"
	returnedFunction2.Config.Spec.Runtime = "r2"

	// verify
	verifyGetFunctions := func(getFunctionsOptions *platform.GetFunctionsOptions) bool {
		suite.Require().Equal("", getFunctionsOptions.Name)
		suite.Require().Equal("f-namespace", getFunctionsOptions.Namespace)

		return true
	}

	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.MatchedBy(verifyGetFunctions)).
		Return([]platform.Function{&returnedFunction1, &returnedFunction2}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-function-namespace": "f-namespace",
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
	"f1": {
		"metadata": {
			"name": "f1",
			"namespace": "f-namespace"
		},
		"spec": {
			"resources": {},
			"build": {},
			"platform": {},
			"runtime": "r1",
		    "eventTimeout": ""
		},
		"status": {}
	},
	"f2": {
		"metadata": {
			"name": "f2",
			"namespace": "f-namespace"
		},
		"spec": {
			"resources": {},
			"build": {},
			"platform": {},
			"runtime": "r2",
		    "eventTimeout": ""
		},
		"status": {}
	}
}`

	suite.sendRequest("GET",
		"/api/functions",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) TestGetListNoNamespace() {
	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Namespace must exist"})
	suite.sendRequest("GET",
		"/api/functions",
		nil,
		nil,
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) TestCreateSuccessful() {

	// verify
	verifyCreateFunction := func(createFunctionOptions *platform.CreateFunctionOptions) bool {
		suite.Require().Equal("f1", createFunctionOptions.FunctionConfig.Meta.Name)
		suite.Require().Equal("f1-namespace", createFunctionOptions.FunctionConfig.Meta.Namespace)
		suite.Require().Equal("proj", createFunctionOptions.FunctionConfig.Meta.Labels["nuclio.io/project-name"])

		return true
	}

	verifyGetFunctions := func(getFunctionsOptions *platform.GetFunctionsOptions) bool {
		suite.Require().Equal("f1", getFunctionsOptions.Name)
		suite.Require().Equal("f1-namespace", getFunctionsOptions.Namespace)
		return true
	}

	suite.mockPlatform.
		On("CreateFunction", mock.Anything, mock.MatchedBy(verifyCreateFunction)).
		Return(&platform.CreateFunctionResult{}, nil).
		Once()

	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.MatchedBy(verifyGetFunctions)).
		Return([]platform.Function{}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-wait-function-action": "true",
		"x-nuclio-project-name":         "proj",
		"x-nuclio-function-namespace":   "f1-namespace",
	}

	expectedStatusCode := http.StatusAccepted
	requestBody := `{
	"metadata": {
		"name": "f1",
		"namespace": "f1-namespace"
	},
	"spec": {
		"resources": {},
		"build": {},
		"platform": {},
		"runtime": "r1"
	}
}`

	suite.sendRequest("POST",
		"/api/functions",
		headers,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		nil)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) TestCreateNoMetadata() {
	suite.sendRequestNoMetadata("POST")
}

func (suite *functionTestSuite) TestCreateNoName() {
	suite.sendRequestNoName("POST")
}

func (suite *functionTestSuite) TestCreateNoNamespace() {
	suite.sendRequestNoNamespace("POST")
}

func (suite *functionTestSuite) TestCreateWithExistingName() {
	suite.sendRequestWithExistingName("POST")
}

func (suite *functionTestSuite) TestCreateFunctionWithInvalidName() {
	body := `{
	"metadata": {
		"namespace": "f1-namespace",
		"name": "!funcmylif&"
	},
	"spec": {
		"resources": {},
		"build": {},
		"platform": {},
		"runtime": "r1"
	}
}`
	headers := map[string]string{
		"x-nuclio-wait-function-action": "true",
	}

	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Function name doesn't conform to k8s naming convention"})
	requestBody := body

	suite.sendRequest("POST",
		"/api/functions",
		headers,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) TestUpdateSuccessful() {
	suite.T().Skip("Update not supported")

	// verify
	verifyUpdateFunction := func(updateFunctionOptions *platform.UpdateFunctionOptions) bool {
		suite.Require().Equal("f1", updateFunctionOptions.FunctionMeta.Name)
		suite.Require().Equal("f1-namespace", updateFunctionOptions.FunctionMeta.Namespace)

		return true
	}

	suite.mockPlatform.
		On("UpdateFunction", mock.Anything, mock.MatchedBy(verifyUpdateFunction)).
		Return(nil).
		Once()

	headers := map[string]string{
		"x-nuclio-wait-function-action": "true",
	}

	expectedStatusCode := http.StatusAccepted
	requestBody := `{
	"metadata": {
		"name": "f1",
		"namespace": "f1-namespace"
	},
	"spec": {
		"resources": {},
		"build": {},
		"platform": {},
		"runtime": "r1"
	}
}`

	suite.sendRequest("PUT",
		"/api/functions",
		headers,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		nil)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) TestUpdateNoMetadata() {
	suite.T().Skip("Update not supported")

	suite.sendRequestNoMetadata("PUT")
}

func (suite *functionTestSuite) TestUpdateNoName() {
	suite.T().Skip("Update not supported")

	suite.sendRequestNoName("PUT")
}

func (suite *functionTestSuite) TestUpdateNoNamespace() {
	suite.T().Skip("Update not supported")

	suite.sendRequestNoNamespace("PUT")
}

func (suite *functionTestSuite) TestDeleteSuccessful() {

	// verify
	verifyDeleteFunction := func(deleteFunctionOptions *platform.DeleteFunctionOptions) bool {
		suite.Require().Equal("f1", deleteFunctionOptions.FunctionConfig.Meta.Name)
		suite.Require().Equal("f1-namespace", deleteFunctionOptions.FunctionConfig.Meta.Namespace)

		return true
	}

	suite.mockPlatform.
		On("DeleteFunction", mock.Anything, mock.MatchedBy(verifyDeleteFunction)).
		Return(nil).
		Once()

	headers := map[string]string{
		"x-nuclio-wait-function-action": "true",
	}

	expectedStatusCode := http.StatusNoContent
	requestBody := `{
	"metadata": {
		"name": "f1",
		"namespace": "f1-namespace"
	}
}`

	suite.sendRequest("DELETE",
		"/api/functions",
		headers,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		nil)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) TestDeleteNoMetadata() {
	suite.sendRequestNoMetadata("DELETE")
}

func (suite *functionTestSuite) TestDeleteNoName() {
	suite.sendRequestNoName("DELETE")
}

func (suite *functionTestSuite) TestDeleteNoNamespace() {
	suite.sendRequestNoNamespace("DELETE")
}

func (suite *functionTestSuite) TestInvokeUnSuccessful() {
	errMessage := "something-bad-happened"
	functionName := "f1"
	functionNamespace := "f1-namespace"

	requestMethod := "PUT"
	requestPath := "/some/path"
	requestBody := []byte("request body")

	// headers we want to pass to the actual function
	functionRequestHeaders := map[string]string{
		"request_h1": "request_h1v",
		"request_h2": "request_h2v",
	}

	// headers we need to pass to dashboard for invocation
	requestHeaders := map[string]string{
		"x-nuclio-path":               requestPath,
		"x-nuclio-function-name":      functionName,
		"x-nuclio-function-namespace": functionNamespace,
		"x-nuclio-invoke-url":         "something-bad",
	}

	// add functionRequestHeaders to requestHeaders so that dashboard will invoke the functions with them
	for headerKey, headerValue := range functionRequestHeaders {
		requestHeaders[headerKey] = headerValue
	}

	// CreateFunctionInvocationResult holds the result of a single invocation
	expectedInvokeResult := platform.CreateFunctionInvocationResult{}

	// verify call to invoke function
	verifyCreateFunctionInvocation := func(createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) bool {
		suite.Require().Equal(functionName, createFunctionInvocationOptions.Name)
		suite.Require().Equal(functionNamespace, createFunctionInvocationOptions.Namespace)
		suite.Require().Equal(requestBody, createFunctionInvocationOptions.Body)
		suite.Require().Equal(requestMethod, createFunctionInvocationOptions.Method)
		suite.Require().Equal(platform.InvokeViaAny, createFunctionInvocationOptions.Via)
		suite.Require().Equal("something-bad", createFunctionInvocationOptions.URL)

		// dashboard will trim the first "/"
		suite.Require().Equal(requestPath[1:], createFunctionInvocationOptions.Path)

		// expect only to receive the function headers (those that don't start with x-nuclio
		for headerKey := range createFunctionInvocationOptions.Headers {
			suite.Require().False(strings.HasPrefix(headerKey, "x-nuclio"))
		}

		// expect all the function headers to be there
		for headerKey, headerValue := range functionRequestHeaders {
			suite.Require().Equal(headerValue, createFunctionInvocationOptions.Headers.Get(headerKey))
		}

		return true
	}

	suite.mockPlatform.
		On("CreateFunctionInvocation", mock.Anything, mock.MatchedBy(verifyCreateFunctionInvocation)).
		Return(&expectedInvokeResult, nuclio.NewErrBadRequest(errMessage)).
		Once()

	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{errMessage})

	suite.sendRequest(requestMethod,
		"/api/function_invocations",
		requestHeaders,
		bytes.NewBuffer(requestBody),
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) TestInvokeSuccessful() {
	functionName := "f1"
	functionNamespace := "f1-namespace"

	requestMethod := "PUT"
	requestPath := "/some/path"
	requestBody := []byte("request body")
	responseBody := []byte(`{"response": "body"}`)

	// headers we want to pass to the actual function
	functionRequestHeaders := map[string]string{
		"request_h1": "request_h1v",
		"request_h2": "request_h2v",
	}

	// headers we need to pass to dashboard for invocation
	requestHeaders := map[string]string{
		"x-nuclio-path":               requestPath,
		"x-nuclio-function-name":      functionName,
		"x-nuclio-function-namespace": functionNamespace,
		"x-nuclio-invoke-via":         "external-ip",
		"x-nuclio-invoke-url":         "something",
		"x-nuclio-invoke-timeout":     "5m",
	}

	// add functionRequestHeaders to requestHeaders so that dashboard will invoke the functions with them
	for headerKey, headerValue := range functionRequestHeaders {
		requestHeaders[headerKey] = headerValue
	}

	// CreateFunctionInvocationResult holds the result of a single invocation
	expectedInvokeResult := platform.CreateFunctionInvocationResult{
		Headers: map[string][]string{
			"response_h1": {"response_h1v"},
			"response_h2": {"response_h2v"},
		},
		Body:       responseBody,
		StatusCode: http.StatusCreated,
	}

	// verify call to invoke function
	verifyCreateFunctionInvocation := func(createFunctionInvocationOptions *platform.CreateFunctionInvocationOptions) bool {
		suite.Require().Equal(functionName, createFunctionInvocationOptions.Name)
		suite.Require().Equal(functionNamespace, createFunctionInvocationOptions.Namespace)
		suite.Require().Equal(requestBody, createFunctionInvocationOptions.Body)
		suite.Require().Equal(requestMethod, createFunctionInvocationOptions.Method)
		suite.Require().Equal(platform.InvokeViaAny, createFunctionInvocationOptions.Via)
		suite.Require().Equal("something", createFunctionInvocationOptions.URL)
		suite.Require().Equal(5*time.Minute, createFunctionInvocationOptions.Timeout)

		// dashboard will trim the first "/"
		suite.Require().Equal(requestPath[1:], createFunctionInvocationOptions.Path)

		// expect only to receive the function headers (those that don't start with x-nuclio
		for headerKey := range createFunctionInvocationOptions.Headers {
			suite.Require().False(strings.HasPrefix(headerKey, "x-nuclio"))
		}

		// expect all the function headers to be there
		for headerKey, headerValue := range functionRequestHeaders {
			suite.Require().Equal(headerValue, createFunctionInvocationOptions.Headers.Get(headerKey))
		}

		return true
	}

	suite.mockPlatform.
		On("CreateFunctionInvocation", mock.Anything, mock.MatchedBy(verifyCreateFunctionInvocation)).
		Return(&expectedInvokeResult, nil).
		Once()

	expectedStatusCode := expectedInvokeResult.StatusCode

	suite.sendRequest(requestMethod,
		"/api/function_invocations",
		requestHeaders,
		bytes.NewBuffer(requestBody),
		&expectedStatusCode,
		string(responseBody))

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) TestInvokeNoName() {

	// headers we need to pass to dashboard for invocation
	requestHeaders := map[string]string{
		"x-nuclio-path":               "p",
		"x-nuclio-function-namespace": "ns",
		"x-nuclio-invoke-via":         "external-ip",
	}

	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Function name must be provided"})

	expectedStatusCode := http.StatusBadRequest
	suite.sendRequest("POST",
		"/api/function_invocations",
		requestHeaders,
		bytes.NewBufferString("request body"),
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) TestInvokeNoNamespace() {

	// headers we need to pass to dashboard for invocation
	requestHeaders := map[string]string{
		"x-nuclio-path":          "p",
		"x-nuclio-function-name": "n",
		"x-nuclio-invoke-via":    "external-ip",
	}

	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Function name must be provided"})

	expectedStatusCode := http.StatusBadRequest
	suite.sendRequest("POST",
		"/api/function_invocations",
		requestHeaders,
		bytes.NewBufferString("request body"),
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) TestExportFunctionSuccessful() {
	replicas := 10
	returnedFunction := platform.AbstractFunction{}
	returnedFunction.Config.Meta.Name = "f1"
	returnedFunction.Config.Meta.Namespace = "f1-namespace"
	returnedFunction.Config.Spec.Replicas = &replicas

	// verify
	verifyGetFunctionsOptions := func(getFunctionsOptions *platform.GetFunctionsOptions) bool {
		suite.Require().Equal("f1", getFunctionsOptions.Name)
		suite.Require().Equal("f1-namespace", getFunctionsOptions.Namespace)

		return true
	}

	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.MatchedBy(verifyGetFunctionsOptions)).
		Return([]platform.Function{&returnedFunction}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-function-namespace": "f1-namespace",
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
	"metadata": {
		"name": "f1",
		"annotations": {
			"skip-build": "true",
			"skip-deploy": "true"
		}
	},
	"spec": {
		"resources": {},
		"build": {},
		"platform": {},
		"replicas": 10,
		"eventTimeout": ""
	}
}`

	suite.sendRequest("GET",
		"/api/functions/f1?export=true",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) TestExportFunctionListSuccessful() {
	replicas := 10
	returnedFunction1 := platform.AbstractFunction{}
	returnedFunction1.Config.Meta.Name = "f1"
	returnedFunction1.Config.Meta.Namespace = "f-namespace"
	returnedFunction1.Config.Spec.Replicas = &replicas

	returnedFunction2 := platform.AbstractFunction{}
	returnedFunction2.Config.Meta.Name = "f2"
	returnedFunction2.Config.Meta.Namespace = "f-namespace"
	returnedFunction2.Config.Spec.Replicas = &replicas

	// verify
	verifyGetFunctionsOptions := func(getFunctionsOptions *platform.GetFunctionsOptions) bool {
		suite.Require().Equal("", getFunctionsOptions.Name)
		suite.Require().Equal("f-namespace", getFunctionsOptions.Namespace)

		return true
	}

	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.MatchedBy(verifyGetFunctionsOptions)).
		Return([]platform.Function{&returnedFunction1, &returnedFunction2}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-function-namespace": "f-namespace",
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
	"f1": {
		"metadata": {
			"name": "f1",
			"annotations": {
				"skip-build": "true",
				"skip-deploy": "true"
			}
		},
		"spec": {
			"resources": {},
			"build": {},
			"platform": {},
			"replicas": 10,
			"eventTimeout": ""
		}
	},
	"f2": {
		"metadata": {
			"name": "f2",
			"annotations": {
				"skip-build": "true",
				"skip-deploy": "true"
			}
		},
		"spec": {
			"resources": {},
			"build": {},
			"platform": {},
			"replicas": 10,
			"eventTimeout": ""
		}
	}
}`

	suite.sendRequest("GET",
		"/api/functions/?export=true",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionTestSuite) sendRequestNoMetadata(method string) {
	suite.sendRequestWithInvalidBody(method, `{
	"spec": {
		"resources": {},
		"build": {},
		"platform": {},
		"runtime": "r1"
	}
}`)
}

func (suite *functionTestSuite) sendRequestNoNamespace(method string) {
	suite.sendRequestWithInvalidBody(method, `{
	"metadata": {
		"name": "f1Name"
	},
	"spec": {
		"resources": {},
		"build": {},
		"platform": {},
		"runtime": "r1"
	}
}`)
}

func (suite *functionTestSuite) sendRequestWithExistingName(method string) {
	returnedFunction := platform.AbstractFunction{}
	returnedFunction.Config.Meta.Name = "f1"
	returnedFunction.Config.Meta.Namespace = "f1-namespace"

	verifyGetFunctions := func(getFunctionsOptions *platform.GetFunctionsOptions) bool {
		suite.Require().Equal("f1", getFunctionsOptions.Name)
		suite.Require().Equal("f1-namespace", getFunctionsOptions.Namespace)
		return true
	}
	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.MatchedBy(verifyGetFunctions)).
		Return([]platform.Function{&returnedFunction}, nil).
		Once()

	expectedStatusCode := http.StatusConflict

	headers := map[string]string{
		"x-nuclio-project-name":       "proj",
		"x-nuclio-function-namespace": "f1-namespace",
	}

	requestBody := `{
	"metadata": {
		"name": "f1",
		"namespace": "f1-namespace"
	},
	"spec": {
		"resources": {},
		"build": {},
		"platform": {},
		"runtime": "r1"
	}
}`

	suite.sendRequest(method,
		"/api/functions",
		headers,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		nil)
}

func (suite *functionTestSuite) sendRequestNoName(method string) {
	suite.sendRequestWithInvalidBody(method, `{
	"metadata": {
		"namespace": "f1-namespace"
	},
	"spec": {
		"resources": {},
		"build": {},
		"platform": {},
		"runtime": "r1"
	}
}`)
}

func (suite *functionTestSuite) sendRequestWithInvalidBody(method string, body string) {
	headers := map[string]string{
		"x-nuclio-wait-function-action": "true",
	}

	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{
		"Function name must be provided in metadata",
		"Function namespace must be provided in metadata",
	})
	requestBody := body

	suite.sendRequest(method,
		"/api/functions",
		headers,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

//
// Project
//

type projectTestSuite struct {
	dashboardTestSuite
}

func (suite *projectTestSuite) TestGetDetailSuccessful() {
	returnedProject := platform.AbstractProject{}
	returnedProject.ProjectConfig.Meta.Name = "p1"
	returnedProject.ProjectConfig.Meta.Namespace = "p1-namespace"
	returnedProject.ProjectConfig.Spec.Description = "p1Desc"

	// verify
	verifyGetProjects := func(getProjectsOptions *platform.GetProjectsOptions) bool {
		suite.Require().Equal("p1", getProjectsOptions.Meta.Name)
		suite.Require().Equal("p1-namespace", getProjectsOptions.Meta.Namespace)

		return true
	}

	suite.mockPlatform.
		On("GetProjects", mock.Anything, mock.MatchedBy(verifyGetProjects)).
		Return([]platform.Project{&returnedProject}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-project-namespace": "p1-namespace",
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
	"metadata": {
		"name": "p1",
		"namespace": "p1-namespace"
	},
	"spec": {
		"description": "p1Desc"
	},
    "status": {}
}`

	suite.sendRequest("GET",
		"/api/projects/p1",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *projectTestSuite) TestGetDetailNoNamespace() {
	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Namespace must exist"})
	suite.sendRequest("GET",
		"/api/projects/p1",
		nil,
		nil,
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *projectTestSuite) TestGetListSuccessful() {
	returnedProject1 := platform.AbstractProject{}
	returnedProject1.ProjectConfig.Meta.Name = "p1"
	returnedProject1.ProjectConfig.Meta.Namespace = "p-namespace"
	returnedProject1.ProjectConfig.Spec.Description = "p1Desc"

	returnedProject2 := platform.AbstractProject{}
	returnedProject2.ProjectConfig.Meta.Name = "p2"
	returnedProject2.ProjectConfig.Meta.Namespace = "p-namespace"
	returnedProject2.ProjectConfig.Spec.Description = "p2Desc"

	// verify
	verifyGetProjects := func(getProjectsOptions *platform.GetProjectsOptions) bool {
		suite.Require().Equal("", getProjectsOptions.Meta.Name)
		suite.Require().Equal("p-namespace", getProjectsOptions.Meta.Namespace)

		return true
	}

	suite.mockPlatform.
		On("GetProjects", mock.Anything, mock.MatchedBy(verifyGetProjects)).
		Return([]platform.Project{&returnedProject1, &returnedProject2}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-project-namespace": "p-namespace",
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
	"p1": {
		"metadata": {
			"name": "p1",
			"namespace": "p-namespace"
		},
		"spec": {
			"description": "p1Desc"
		},
        "status": {}
	},
	"p2": {
		"metadata": {
			"name": "p2",
			"namespace": "p-namespace"
		},
		"spec": {
			"description": "p2Desc"
		},
        "status": {}
	}
}`

	suite.sendRequest("GET",
		"/api/projects",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *projectTestSuite) TestGetListNoNamespace() {
	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Namespace must exist"})
	suite.sendRequest("GET",
		"/api/projects",
		nil,
		nil,
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *projectTestSuite) TestExportProjectSuccessful() {
	returnedFunction1 := platform.AbstractFunction{}
	returnedFunction1.Config.Meta.Name = "f1"
	returnedFunction1.Config.Meta.Namespace = "f-namespace"
	returnedFunction1.Config.Spec.Runtime = "r1"

	returnedFunctionEvent := platform.AbstractFunctionEvent{}
	returnedFunctionEvent.FunctionEventConfig.Meta.Name = "fe1"
	returnedFunctionEvent.FunctionEventConfig.Meta.Namespace = "f-namespace"
	returnedFunctionEvent.FunctionEventConfig.Meta.Labels = map[string]string{"nuclio.io/function-name": "f1"}
	returnedFunctionEvent.FunctionEventConfig.Spec.DisplayName = "fe1DisplayName"
	returnedFunctionEvent.FunctionEventConfig.Spec.TriggerName = "fe1TriggerName"
	returnedFunctionEvent.FunctionEventConfig.Spec.TriggerKind = "fe1TriggerKind"
	returnedFunctionEvent.FunctionEventConfig.Spec.Body = "fe1Body"

	returnedFunction2 := platform.AbstractFunction{}
	returnedFunction2.Config.Meta.Name = "f2"
	returnedFunction2.Config.Meta.Namespace = "f-namespace"
	returnedFunction2.Config.Spec.Runtime = "r2"

	returnedProject1 := platform.AbstractProject{}
	returnedProject1.ProjectConfig.Meta.Name = "p1"
	returnedProject1.ProjectConfig.Meta.Namespace = "f-namespace"

	returnedAPIGateway1 := platform.AbstractAPIGateway{}
	returnedAPIGateway1.APIGatewayConfig.Meta.Name = "agw1"
	returnedAPIGateway1.APIGatewayConfig.Meta.Namespace = "f-namespace"

	verifyGetAPIGateways := func(getAPIGatewaysOptions *platform.GetAPIGatewaysOptions) bool {
		suite.Require().Equal("", getAPIGatewaysOptions.Name)
		suite.Require().Equal("f-namespace", getAPIGatewaysOptions.Namespace)

		return true
	}

	// verify
	verifyGetProjects := func(getProjectsOptions *platform.GetProjectsOptions) bool {
		suite.Require().Equal("p1", getProjectsOptions.Meta.Name)
		suite.Require().Equal("f-namespace", getProjectsOptions.Meta.Namespace)

		return true
	}
	verifyGetFunctions := func(getFunctionsOptions *platform.GetFunctionsOptions) bool {
		suite.Require().Equal("", getFunctionsOptions.Name)
		suite.Require().Equal("f-namespace", getFunctionsOptions.Namespace)

		return true
	}
	verifyGetFunction1Events := func(getFunctionEventsOptions *platform.GetFunctionEventsOptions) bool {
		suite.Require().Equal("", getFunctionEventsOptions.Meta.Name)
		suite.Require().Equal("f-namespace", getFunctionEventsOptions.Meta.Namespace)

		return true
	}
	verifyGetFunction2Events := func(getFunctionEventsOptions *platform.GetFunctionEventsOptions) bool {
		suite.Require().Equal("", getFunctionEventsOptions.Meta.Name)
		suite.Require().Equal("f-namespace", getFunctionEventsOptions.Meta.Namespace)

		return true
	}

	suite.mockPlatform.
		On("GetProjects", mock.Anything, mock.MatchedBy(verifyGetProjects)).
		Return([]platform.Project{&returnedProject1}, nil).
		Once()

	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.MatchedBy(verifyGetFunctions)).
		Return([]platform.Function{&returnedFunction1, &returnedFunction2}, nil).
		Once()

	suite.mockPlatform.
		On("GetFunctionEvents", mock.Anything, mock.MatchedBy(verifyGetFunction1Events)).
		Return([]platform.FunctionEvent{&returnedFunctionEvent}, nil).Once()

	suite.mockPlatform.
		On("GetFunctionEvents", mock.Anything, mock.MatchedBy(verifyGetFunction2Events)).
		Return([]platform.FunctionEvent{}, nil).Once()

	suite.mockPlatform.
		On("GetAPIGateways", mock.Anything, mock.MatchedBy(verifyGetAPIGateways)).
		Return([]platform.APIGateway{&returnedAPIGateway1}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-project-namespace":  "f-namespace",
		"x-nuclio-function-namespace": "f-namespace",
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
  "apiGateways": {
    "agw1": {
      "metadata": {
        "name": "agw1"
      },
      "spec": {}
    }
  },
  "functionEvents": {
    "fe1": {
      "metadata": {
        "name": "fe1",
        "namespace": "f-namespace",
        "labels": {
          "nuclio.io/function-name": "f1"
        }
      },
      "spec": {
        "displayName": "fe1DisplayName",
        "triggerName": "fe1TriggerName",
        "triggerKind": "fe1TriggerKind",
        "body": "fe1Body"
      }
    }
  },
  "functions": {
    "f1": {
      "metadata": {
        "name": "f1",
        "annotations": {
          "skip-build": "true",
          "skip-deploy": "true"
        }
      },
      "spec": {
        "runtime": "r1",
        "resources": {},
        "build": {},
        "platform": {},
        "eventTimeout": ""
      }
    },
    "f2": {
      "metadata": {
        "name": "f2",
        "annotations": {
          "skip-build": "true",
          "skip-deploy": "true"
        }
      },
      "spec": {
        "runtime": "r2",
        "resources": {},
        "build": {},
        "platform": {},
        "eventTimeout": ""
      }
    }
  },
  "project": {
    "metadata": {
      "name": "p1"
    },
    "spec": {},
    "status": {}
  }
}`

	suite.sendRequest("GET",
		"/api/projects/p1?export=true",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *projectTestSuite) TestExportProjectListSuccessful() {
	returnedFunction1 := platform.AbstractFunction{}
	returnedFunction1.Config.Meta.Name = "f1"
	returnedFunction1.Config.Meta.Namespace = "f-namespace"
	returnedFunction1.Config.Spec.Runtime = "r1"

	returnedFunction2 := platform.AbstractFunction{}
	returnedFunction2.Config.Meta.Name = "f2"
	returnedFunction2.Config.Meta.Namespace = "f-namespace"
	returnedFunction2.Config.Spec.Runtime = "r2"

	returnedProject1 := platform.AbstractProject{}
	returnedProject1.ProjectConfig.Meta.Name = "p1"
	returnedProject1.ProjectConfig.Meta.Namespace = "f-namespace"

	returnedProject2 := platform.AbstractProject{}
	returnedProject2.ProjectConfig.Meta.Name = "p2"
	returnedProject2.ProjectConfig.Meta.Namespace = "f-namespace"

	returnedAPIGateway1 := platform.AbstractAPIGateway{}
	returnedAPIGateway1.APIGatewayConfig.Meta.Name = "agw1"
	returnedAPIGateway1.APIGatewayConfig.Meta.Namespace = "f-namespace"

	returnedAPIGateway2 := platform.AbstractAPIGateway{}
	returnedAPIGateway2.APIGatewayConfig.Meta.Name = "agw2"
	returnedAPIGateway2.APIGatewayConfig.Meta.Namespace = "f-namespace"

	// verify
	verifyGetProjects := func(getProjectsOptions *platform.GetProjectsOptions) bool {
		suite.Require().Equal("", getProjectsOptions.Meta.Name)
		suite.Require().Equal("f-namespace", getProjectsOptions.Meta.Namespace)

		return true
	}
	verifyGetFunctions := func(getFunctionsOptions *platform.GetFunctionsOptions) bool {
		suite.Require().Equal("", getFunctionsOptions.Name)
		suite.Require().Equal("f-namespace", getFunctionsOptions.Namespace)

		return true
	}
	verifyGetAPIGateways := func(getAPIGatewaysOptions *platform.GetAPIGatewaysOptions) bool {
		suite.Require().Equal("", getAPIGatewaysOptions.Name)
		suite.Require().Equal("f-namespace", getAPIGatewaysOptions.Namespace)

		return true
	}
	verifyGetFunctionEvents := func(getFunctionEventsOptions *platform.GetFunctionEventsOptions) bool {
		suite.Require().Equal("f-namespace", getFunctionEventsOptions.Meta.Namespace)

		return true
	}

	suite.mockPlatform.
		On("GetProjects", mock.Anything, mock.MatchedBy(verifyGetProjects)).
		Return([]platform.Project{&returnedProject1, &returnedProject2}, nil).
		Once()

	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.MatchedBy(verifyGetFunctions)).
		Return([]platform.Function{&returnedFunction1}, nil).
		Once()

	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.MatchedBy(verifyGetFunctions)).
		Return([]platform.Function{&returnedFunction2}, nil).
		Once()

	suite.mockPlatform.
		On("GetAPIGateways", mock.Anything, mock.MatchedBy(verifyGetAPIGateways)).
		Return([]platform.APIGateway{&returnedAPIGateway1}, nil).
		Once()

	suite.mockPlatform.
		On("GetAPIGateways", mock.Anything, mock.MatchedBy(verifyGetAPIGateways)).
		Return([]platform.APIGateway{&returnedAPIGateway2}, nil).
		Once()

	suite.mockPlatform.
		On("GetFunctionEvents", mock.Anything, mock.MatchedBy(verifyGetFunctionEvents)).
		Return([]platform.FunctionEvent{}, nil).Twice()

	headers := map[string]string{
		"x-nuclio-project-namespace":  "f-namespace",
		"x-nuclio-function-namespace": "f-namespace",
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
  "p1": {
    "apiGateways": {
      "agw1": {
        "metadata": {
          "name": "agw1"
        },
        "spec": {}
      }
    },
    "functionEvents": {},
    "functions": {
      "f1": {
        "metadata": {
          "name": "f1",
          "annotations": {
            "skip-build": "true",
            "skip-deploy": "true"
          }
        },
        "spec": {
          "runtime": "r1",
          "resources": {},
          "build": {},
          "platform": {},
          "eventTimeout": ""
        }
      }
    },
    "project": {
      "metadata": {
        "name": "p1"
      },
      "spec": {},
	  "status": {}
    }
  },
  "p2": {
    "apiGateways": {
      "agw2": {
        "metadata": {
          "name": "agw2"
        },
        "spec": {}
      }
    },
    "functionEvents": {},
    "functions": {
      "f2": {
        "metadata": {
          "name": "f2",
          "annotations": {
            "skip-build": "true",
            "skip-deploy": "true"
          }
        },
        "spec": {
          "runtime": "r2",
          "resources": {},
          "build": {},
          "platform": {},
          "eventTimeout": ""
        }
      }
    },
    "project": {
      "metadata": {
        "name": "p2"
      },
      "spec": {},
      "status": {}
    }
  }
}`

	suite.sendRequest("GET",
		"/api/projects/?export=true",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *projectTestSuite) TestCreateSuccessful() {

	// verify
	verifyCreateProject := func(createProjectOptions *platform.CreateProjectOptions) bool {
		suite.Require().Equal("p1", createProjectOptions.ProjectConfig.Meta.Name)
		suite.Require().Equal("p1-namespace", createProjectOptions.ProjectConfig.Meta.Namespace)
		suite.Require().Equal("p1Description", createProjectOptions.ProjectConfig.Spec.Description)

		return true
	}

	suite.mockPlatform.
		On("CreateProject", mock.Anything, mock.MatchedBy(verifyCreateProject)).
		Return(nil).
		Once()

	expectedStatusCode := http.StatusCreated
	requestBody := `{
	"metadata": {
		"name": "p1",
		"namespace": "p1-namespace"
	},
	"spec": {
		"description": "p1Description"
	}
}`
	expectedResponseBody := `{
	"metadata": {
		"name": "p1",
		"namespace": "p1-namespace"
	},
	"spec": {
		"description": "p1Description"
	},
    "status": {}
}`

	suite.sendRequest("POST",
		"/api/projects",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *projectTestSuite) TestCreateNoMetadata() {
	suite.sendRequestNoMetadata("POST")
}

func (suite *projectTestSuite) TestCreateNoName() {
	suite.mockPlatform.
		On("CreateProject", mock.Anything, mock.Anything).
		Return(nil).
		Once()

	expectedStatusCode := http.StatusCreated
	requestBody := `{
	"metadata": {
		"name": "p1name",
		"namespace": "p1-namespace"
	},
	"spec": {
		"description": "p1Description"
	}
}`

	responseVerifier := func(response map[string]interface{}) bool {

		// get metadata as a map
		metadata := response["metadata"].(map[string]interface{})

		// get name
		name := metadata["name"].(string)
		suite.NotEqual("", name)

		return true
	}

	suite.sendRequest("POST",
		"/api/projects",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		responseVerifier)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *projectTestSuite) TestCreateNoNamespace() {
	suite.sendRequestNoNamespace("POST")
}

func (suite *projectTestSuite) TestUpdateSuccessful() {

	// verify
	verifyUpdateProject := func(updateProjectOptions *platform.UpdateProjectOptions) bool {
		suite.Require().Equal("p1", updateProjectOptions.ProjectConfig.Meta.Name)
		suite.Require().Equal("p1-namespace", updateProjectOptions.ProjectConfig.Meta.Namespace)
		suite.Require().Equal("p1Description", updateProjectOptions.ProjectConfig.Spec.Description)

		return true
	}

	suite.mockPlatform.
		On("UpdateProject", mock.Anything, mock.MatchedBy(verifyUpdateProject)).
		Return(nil).
		Once()

	expectedStatusCode := http.StatusNoContent
	requestBody := `{
	"metadata": {
		"name": "p1",
		"namespace": "p1-namespace"
	},
	"spec": {
		"description": "p1Description"
	}
}`

	suite.sendRequest("PUT",
		"/api/projects",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		nil)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *projectTestSuite) TestUpdateNoMetadata() {
	suite.sendRequestNoMetadata("PUT")
}

func (suite *projectTestSuite) TestUpdateNoName() {
	suite.sendRequestNoName("PUT")
}

func (suite *projectTestSuite) TestUpdateNoNamespace() {
	suite.sendRequestNoNamespace("PUT")
}

func (suite *projectTestSuite) TestDeleteSuccessful() {

	// verify
	verifyDeleteProject := func(deleteProjectOptions *platform.DeleteProjectOptions) bool {
		suite.Require().Equal("p1", deleteProjectOptions.Meta.Name)
		suite.Require().Equal("p1-namespace", deleteProjectOptions.Meta.Namespace)

		return true
	}

	suite.mockPlatform.
		On("DeleteProject", mock.Anything, mock.MatchedBy(verifyDeleteProject)).
		Return(nil).
		Once()

	expectedStatusCode := http.StatusNoContent
	requestBody := `{
	"metadata": {
		"name": "p1",
		"namespace": "p1-namespace"
	}
}`

	suite.sendRequest("DELETE",
		"/api/projects",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		nil)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *projectTestSuite) TestDeleteWithFunctions() {
	for _, testCase := range []struct {
		name                       string
		expectedStatusCode         int
		deleteProjectReturnedError error
		deleteProjectOptions       *platform.DeleteProjectOptions
		requestHeaders             map[string]string
	}{
		{
			name: "DeleteProjectWithFunctions",
			deleteProjectOptions: &platform.DeleteProjectOptions{
				Meta: platform.ProjectMeta{
					Name:      "p1",
					Namespace: "p1-namespace",
				},
				Strategy: platform.DeleteProjectStrategyCascading,
			},
			deleteProjectReturnedError: nil,
			requestHeaders: map[string]string{
				"x-nuclio-delete-project-strategy": string(platform.DeleteProjectStrategyCascading),
			},
			expectedStatusCode: http.StatusNoContent,
		},
		{
			name: "FailDeleteProjectWithFunctions",
			deleteProjectOptions: &platform.DeleteProjectOptions{
				Meta: platform.ProjectMeta{
					Name:      "p1",
					Namespace: "p1-namespace",
				},
				Strategy: platform.DeleteProjectStrategyRestricted,
			},
			requestHeaders: map[string]string{
				"x-nuclio-delete-project-strategy": string(platform.DeleteProjectStrategyRestricted),
			},
			deleteProjectReturnedError: nuclio.NewErrPreconditionFailed("functions exists"),
			expectedStatusCode:         http.StatusPreconditionFailed,
		},
		{
			name: "DefaultDeleteProjectStrategyToRestricted",
			deleteProjectOptions: &platform.DeleteProjectOptions{
				Meta: platform.ProjectMeta{
					Name:      "p1",
					Namespace: "p1-namespace",
				},
				Strategy: platform.DeleteProjectStrategyRestricted,
			},
			requestHeaders:             map[string]string{},
			deleteProjectReturnedError: nil,
			expectedStatusCode:         http.StatusNoContent,
		},
	} {
		suite.Run(testCase.name, func() {
			testCase.deleteProjectOptions.AuthSession = &auth.NopSession{}
			suite.mockPlatform.
				On("DeleteProject", mock.Anything, testCase.deleteProjectOptions).
				Return(testCase.deleteProjectReturnedError).
				Once()

			requestBody := fmt.Sprintf(`{"metadata": {"name": "%s", "namespace": "%s"}}`,
				testCase.deleteProjectOptions.Meta.Name,
				testCase.deleteProjectOptions.Meta.Namespace)

			suite.sendRequest("DELETE",
				"/api/projects",
				testCase.requestHeaders,
				bytes.NewBufferString(requestBody),
				&testCase.expectedStatusCode,
				nil)
		})
	}

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *projectTestSuite) TestDeleteNoMetadata() {
	suite.sendRequestNoMetadata("DELETE")
}

func (suite *projectTestSuite) TestDeleteNoName() {
	suite.sendRequestNoName("DELETE")
}

func (suite *projectTestSuite) TestDeleteNoNamespace() {
	suite.sendRequestNoNamespace("DELETE")
}

func (suite *projectTestSuite) TestImportSuccessful() {
	createdProject := platform.AbstractProject{}
	createdProject.ProjectConfig.Meta.Name = "p1"
	createdProject.ProjectConfig.Meta.Namespace = "p1-namespace"
	createdProject.ProjectConfig.Spec.Description = "p1Description"

	// verify
	verifyGetProjects := func(getProjectsOptions *platform.GetProjectsOptions) bool {
		suite.Require().Equal("p1", getProjectsOptions.Meta.Name)
		suite.Require().Equal("p1-namespace", getProjectsOptions.Meta.Namespace)

		return true
	}
	verifyCreateProject := func(createProjectOptions *platform.CreateProjectOptions) bool {
		suite.Require().Equal("p1", createProjectOptions.ProjectConfig.Meta.Name)
		suite.Require().Equal("p1-namespace", createProjectOptions.ProjectConfig.Meta.Namespace)
		suite.Require().Equal("p1Description", createProjectOptions.ProjectConfig.Spec.Description)

		return true
	}
	verifyCreateFunction := func(createFunctionOptions *platform.CreateFunctionOptions) bool {
		suite.Require().Equal("f1", createFunctionOptions.FunctionConfig.Meta.Name)
		suite.Require().Equal("p1-namespace", createFunctionOptions.FunctionConfig.Meta.Namespace)
		suite.Require().Equal("p1", createFunctionOptions.FunctionConfig.Meta.Labels["nuclio.io/project-name"])

		return true
	}
	verifyGetFunctions := func(getFunctionsOptions *platform.GetFunctionsOptions) bool {
		suite.Require().Equal("f1", getFunctionsOptions.Name)
		suite.Require().Equal("p1-namespace", getFunctionsOptions.Namespace)
		return true
	}
	verifyCreateFunctionEvent := func(createFunctionOptions *platform.CreateFunctionEventOptions) bool {
		suite.Require().NotEqual("fe1", createFunctionOptions.FunctionEventConfig.Meta.Name)
		suite.Require().Equal("p1-namespace", createFunctionOptions.FunctionEventConfig.Meta.Namespace)
		suite.Require().Equal("f1", createFunctionOptions.FunctionEventConfig.Meta.Labels["nuclio.io/function-name"])

		return true
	}

	verifyCreateAPIGateway := func(createAPIGatewayOptions *platform.CreateAPIGatewayOptions) bool {
		suite.Require().Equal("agw1", createAPIGatewayOptions.APIGatewayConfig.Meta.Name)
		suite.Require().Equal("p1-namespace", createAPIGatewayOptions.APIGatewayConfig.Meta.Namespace)
		suite.Require().Equal("some-host", createAPIGatewayOptions.APIGatewayConfig.Spec.Host)
		suite.Require().Equal(platform.APIGatewayUpstreamKindNuclioFunction, createAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[0].Kind)
		suite.Require().Equal("f1", createAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[0].NuclioFunction.Name)

		return true
	}

	suite.mockPlatform.
		On("GetProjects", mock.Anything, mock.MatchedBy(verifyGetProjects)).
		Return([]platform.Project{}, nil).
		Once()

	suite.mockPlatform.
		On("CreateProject", mock.Anything, mock.MatchedBy(verifyCreateProject)).
		Return(nil).
		Once()

	suite.mockPlatform.
		On("GetProjects", mock.Anything, mock.MatchedBy(verifyGetProjects)).
		Return([]platform.Project{&createdProject}, nil).
		Twice()

	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.MatchedBy(verifyGetFunctions)).
		Return([]platform.Function{}, nil).
		Once()

	suite.mockPlatform.
		On("CreateFunction", mock.Anything, mock.MatchedBy(verifyCreateFunction)).
		Return(&platform.CreateFunctionResult{}, nil).
		Once()

	suite.mockPlatform.
		On("CreateFunctionEvent", mock.Anything, mock.MatchedBy(verifyCreateFunctionEvent)).
		Return(nil).
		Once()

	suite.mockPlatform.
		On("CreateAPIGateway", mock.Anything, mock.MatchedBy(verifyCreateAPIGateway)).
		Return(nil).
		Once()

	headers := map[string]string{
		"x-nuclio-wait-function-action": "true",
	}

	expectedStatusCode := http.StatusCreated
	requestBody := `{
  "project": {
    "metadata": {
      "name": "p1",
      "namespace": "p1-namespace"
    },
    "spec": {
      "description": "p1Description"
    }
  },
  "functions": {
    "f1": {
      "metadata": {
        "name": "f1",
        "namespace": "p1-namespace"
      },
      "spec": {
        "resources": {},
        "build": {},
        "platform": {},
        "runtime": "r1"
      }
    }
  },
  "functionEvents": {
    "fe1": {
      "metadata": {
        "name": "fe1",
        "namespace": "p1-namespace",
        "labels": {
          "nuclio.io/function-name": "f1"
        }
      },
      "spec": {
        "displayName": "fe1DisplayName",
        "triggerName": "fe1TriggerName",
        "triggerKind": "fe1TriggerKind",
        "body": "fe1Body"
      }
    }
  },
  "apiGateways": {
    "agw1": {
      "metadata": {
        "name": "agw1",
        "namespace": "p1-namespace"
      },
      "spec": {
        "name": "agw1",
        "host": "some-host",
        "upstreams": [
          {
            "kind": "nucliofunction",
            "nucliofunction": {
              "name": "f1"
            }
          }
        ]
      }
    }
  }
}`

	expectedResponseBody := `{
  "apiGatewayImportResult": {
    "createdAmount": 1,
    "failedAPIGateways": null,
    "failedAmount": 0
  },
  "functionEventImportResult": {
    "createdAmount": 1,
    "failedAmount": 0,
    "failedFunctionEvents": null
  },
  "functionImportResult": {
    "createdAmount": 1,
    "failedAmount": 0,
    "failedFunctions": null
  }
}`

	suite.sendRequest("POST",
		"/api/projects?import=true",
		headers,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *projectTestSuite) TestImportFunctionExistsSuccessful() {
	existingFunction1 := platform.AbstractFunction{}
	existingFunction1.Config.Meta.Name = "f1"
	existingFunction1.Config.Meta.Namespace = "p1-namespace"
	existingFunction1.Config.Spec.Runtime = "r1"

	createdProject := platform.AbstractProject{}
	createdProject.ProjectConfig.Meta.Name = "p1"
	createdProject.ProjectConfig.Meta.Namespace = "p1-namespace"
	createdProject.ProjectConfig.Spec.Description = "p1Description"

	apiGateway := platform.AbstractAPIGateway{}
	apiGateway.APIGatewayConfig.Meta.Name = "agw1"
	apiGateway.APIGatewayConfig.Meta.Namespace = "p1-namespace"
	apiGateway.APIGatewayConfig.Spec.Host = "host-name1"
	apiGateway.APIGatewayConfig.Spec.Upstreams = []platform.APIGatewayUpstreamSpec{
		{
			Kind: platform.APIGatewayUpstreamKindNuclioFunction,
			NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
				Name: "f1",
			},
		},
	}

	// verify
	verifyGetProjects := func(getProjectsOptions *platform.GetProjectsOptions) bool {
		suite.Require().Equal("p1", getProjectsOptions.Meta.Name)
		suite.Require().Equal("p1-namespace", getProjectsOptions.Meta.Namespace)

		return true
	}
	verifyCreateProject := func(createProjectOptions *platform.CreateProjectOptions) bool {
		suite.Require().Equal("p1", createProjectOptions.ProjectConfig.Meta.Name)
		suite.Require().Equal("p1-namespace", createProjectOptions.ProjectConfig.Meta.Namespace)
		suite.Require().Equal("p1Description", createProjectOptions.ProjectConfig.Spec.Description)

		return true
	}
	verifyGetFunctions := func(getFunctionsOptions *platform.GetFunctionsOptions) bool {
		suite.Require().Equal("f1", getFunctionsOptions.Name)
		suite.Require().Equal("p1-namespace", getFunctionsOptions.Namespace)
		return true
	}

	verifyCreateAPIGateway := func(createAPIGatewayOptions *platform.CreateAPIGatewayOptions) bool {
		suite.Require().Equal("agw1", createAPIGatewayOptions.APIGatewayConfig.Meta.Name)
		suite.Require().Equal("p1-namespace", createAPIGatewayOptions.APIGatewayConfig.Meta.Namespace)
		suite.Require().Equal("host-name1", createAPIGatewayOptions.APIGatewayConfig.Spec.Host)
		suite.Require().Equal(platform.APIGatewayUpstreamKindNuclioFunction, createAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[0].Kind)
		suite.Require().Equal("f1", createAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[0].NuclioFunction.Name)

		return true
	}

	suite.mockPlatform.
		On("GetProjects", mock.Anything, mock.MatchedBy(verifyGetProjects)).
		Return([]platform.Project{}, nil).
		Once()

	suite.mockPlatform.
		On("CreateProject", mock.Anything, mock.MatchedBy(verifyCreateProject)).
		Return(nil).
		Once()

	suite.mockPlatform.
		On("GetProjects", mock.Anything, mock.MatchedBy(verifyGetProjects)).
		Return([]platform.Project{&createdProject}, nil).
		Twice()

	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.MatchedBy(verifyGetFunctions)).
		Return([]platform.Function{&existingFunction1}, nil).
		Once()

	suite.mockPlatform.
		On("CreateAPIGateway", mock.Anything, mock.MatchedBy(verifyCreateAPIGateway)).
		Return(nil).
		Once()

	expectedStatusCode := http.StatusCreated
	requestBody := `{
  "project": {
    "metadata": {
      "name": "p1",
      "namespace": "p1-namespace"
    },
    "spec": {
      "description": "p1Description"
    }
  },
  "functions": {
    "f1": {
      "metadata": {
        "name": "f1",
        "namespace": "p1-namespace"
      },
      "spec": {
        "resources": {},
        "build": {},
        "platform": {},
        "runtime": "r1"
      }
    }
  },
  "functionEvents": {
    "fe1": {
      "metadata": {
        "name": "fe1",
        "namespace": "p1-namespace",
        "labels": {
          "nuclio.io/function-name": "f1"
        }
      },
      "spec": {
        "displayName": "fe1DisplayName",
        "triggerName": "fe1TriggerName",
        "triggerKind": "fe1TriggerKind",
        "body": "fe1Body"
      }
    }
  },
  "apiGateways": {
    "agw1": {
      "metadata": {
        "name": "agw1",
        "namespace": "p1-namespace"
      },
      "spec": {
		"name": "agw1",
        "host": "host-name1",
        "upstreams": [
          {
            "kind": "nucliofunction",
            "nucliofunction": {
              "name": "f1"
            }
          }
        ]
      }
    }
  }
}`

	expectedResponseBody := `{
  "apiGatewayImportResult": {
    "createdAmount": 1,
    "failedAPIGateways": null,
    "failedAmount": 0
  },
  "functionEventImportResult": {
    "createdAmount": 0,
    "failedAmount": 1,
    "failedFunctionEvents": [
      {
        "error": "Event belongs to function that failed import: f1",
        "functionEvent": "fe1DisplayName"
      }
    ]
  },
  "functionImportResult": {
    "createdAmount": 0,
    "failedAmount": 1,
    "failedFunctions": [
      {
        "error": "Function name already exists",
        "function": "f1"
      }
    ]
  }
}`

	suite.sendRequest("POST",
		"/api/projects?import=true",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *projectTestSuite) sendRequestNoMetadata(method string) {
	suite.sendRequestWithInvalidBody(method, `{
	"spec": {
		"description": "d"
	}
}`)
}

func (suite *projectTestSuite) sendRequestNoNamespace(method string) {
	suite.sendRequestWithInvalidBody(method, `{
	"metadata": {
		"name": "name"
	},
	"spec": {
		"description": "d"
	}
}`)
}

func (suite *projectTestSuite) sendRequestNoName(method string) {
	suite.sendRequestWithInvalidBody(method, `{
	"metadata": {
		"namespace": "namespace"
	},
	"spec": {
		"description": "d"
	}
}`)
}

func (suite *projectTestSuite) sendRequestWithInvalidBody(method string, body string) {
	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{
		"Project name must be provided in metadata",
		"Project namespace must be provided in metadata",
	})
	requestBody := body

	response, _ := suite.sendRequest(method,
		"/api/projects",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		ecv.Verify)

	suite.Require().Equal("application/json", response.Header.Get("Content-Type"))

	suite.mockPlatform.AssertExpectations(suite.T())
}

//
// Function event
//

type functionEventTestSuite struct {
	dashboardTestSuite
}

func (suite *functionEventTestSuite) TestGetDetailSuccessful() {
	returnedFunctionEvent := platform.AbstractFunctionEvent{}
	returnedFunctionEvent.FunctionEventConfig.Meta.Name = "fe1"
	returnedFunctionEvent.FunctionEventConfig.Meta.Namespace = "fe1-namespace"
	returnedFunctionEvent.FunctionEventConfig.Meta.Labels = map[string]string{"nuclio.io/function-name": "fe1Func"}
	returnedFunctionEvent.FunctionEventConfig.Spec.DisplayName = "fe1DisplayName"
	returnedFunctionEvent.FunctionEventConfig.Spec.TriggerName = "fe1TriggerName"
	returnedFunctionEvent.FunctionEventConfig.Spec.TriggerKind = "fe1TriggerKind"
	returnedFunctionEvent.FunctionEventConfig.Spec.Body = "fe1Body"
	returnedFunctionEvent.FunctionEventConfig.Spec.Attributes = map[string]interface{}{
		"fe1KeyA": "fe1StringValue",
		"fe1KeyB": []interface{}{"fe1ListValueItemA", "fe1ListValueItemB"},
	}

	// verify
	verifyGetFunctionEvents := func(getFunctionEventsOptions *platform.GetFunctionEventsOptions) bool {
		suite.Require().Equal("fe1", getFunctionEventsOptions.Meta.Name)
		suite.Require().Equal("fe1-namespace", getFunctionEventsOptions.Meta.Namespace)

		return true
	}

	suite.mockPlatform.
		On("GetFunctionEvents", mock.Anything, mock.MatchedBy(verifyGetFunctionEvents)).
		Return([]platform.FunctionEvent{&returnedFunctionEvent}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-function-event-namespace": "fe1-namespace",
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
	"metadata": {
		"name": "fe1",
		"namespace": "fe1-namespace",
		"labels": {
			"nuclio.io/function-name": "fe1Func"
		}
	},
	"spec": {
		"displayName": "fe1DisplayName",
		"triggerName": "fe1TriggerName",
		"triggerKind": "fe1TriggerKind",
		"body": "fe1Body",
		"attributes": {
			"fe1KeyA": "fe1StringValue",
			"fe1KeyB": ["fe1ListValueItemA", "fe1ListValueItemB"]
		}
	}
}`

	suite.sendRequest("GET",
		"/api/function_events/fe1",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionEventTestSuite) TestGetDetailNoNamespace() {
	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Namespace must exist"})
	suite.sendRequest("GET",
		"/api/function_events/fe1",
		nil,
		nil,
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionEventTestSuite) TestGetListSuccessful() {
	returnedFunctionEvent1 := platform.AbstractFunctionEvent{}
	returnedFunctionEvent1.FunctionEventConfig.Meta.Name = "fe1"
	returnedFunctionEvent1.FunctionEventConfig.Meta.Namespace = "fe-namespace"
	returnedFunctionEvent1.FunctionEventConfig.Meta.Labels = map[string]string{"nuclio.io/function-name": "feFunc"}
	returnedFunctionEvent1.FunctionEventConfig.Spec.DisplayName = "fe1DisplayName"
	returnedFunctionEvent1.FunctionEventConfig.Spec.TriggerName = "fe1TriggerName"
	returnedFunctionEvent1.FunctionEventConfig.Spec.TriggerKind = "fe1TriggerKind"
	returnedFunctionEvent1.FunctionEventConfig.Spec.Body = "fe1Body"
	returnedFunctionEvent1.FunctionEventConfig.Spec.Attributes = map[string]interface{}{
		"fe1KeyA": "fe1StringValue",
		"fe1KeyB": []interface{}{"fe1ListValueItemA", "fe1ListValueItemB"},
	}

	returnedFunctionEvent2 := platform.AbstractFunctionEvent{}
	returnedFunctionEvent2.FunctionEventConfig.Meta.Name = "fe2"
	returnedFunctionEvent2.FunctionEventConfig.Meta.Namespace = "fe-namespace"
	returnedFunctionEvent2.FunctionEventConfig.Meta.Labels = map[string]string{"nuclio.io/function-name": "feFunc"}
	returnedFunctionEvent2.FunctionEventConfig.Spec.DisplayName = "fe2DisplayName"
	returnedFunctionEvent2.FunctionEventConfig.Spec.TriggerName = "fe2TriggerName"
	returnedFunctionEvent2.FunctionEventConfig.Spec.TriggerKind = "fe2TriggerKind"
	returnedFunctionEvent2.FunctionEventConfig.Spec.Body = "fe2Body"
	returnedFunctionEvent2.FunctionEventConfig.Spec.Attributes = map[string]interface{}{
		"fe2KeyA": "fe2StringValue",
		"fe2KeyB": []interface{}{"fe2ListValueItemA", "fe2ListValueItemB"},
	}
	// verify
	verifyGetFunctionEvents := func(getFunctionEventsOptions *platform.GetFunctionEventsOptions) bool {
		suite.Require().Equal("", getFunctionEventsOptions.Meta.Name)
		suite.Require().Equal("fe-namespace", getFunctionEventsOptions.Meta.Namespace)
		suite.Require().Equal("feFunc", getFunctionEventsOptions.Meta.Labels["nuclio.io/function-name"])

		return true
	}

	suite.mockPlatform.
		On("GetFunctionEvents", mock.Anything, mock.MatchedBy(verifyGetFunctionEvents)).
		Return([]platform.FunctionEvent{&returnedFunctionEvent1, &returnedFunctionEvent2}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-function-event-namespace": "fe-namespace",
		"x-nuclio-function-name":            "feFunc",
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
	"fe1": {
		"metadata": {
			"name": "fe1",
			"namespace": "fe-namespace",
			"labels": {
				"nuclio.io/function-name": "feFunc"
			}
		},
		"spec": {
			"displayName": "fe1DisplayName",
			"triggerName": "fe1TriggerName",
			"triggerKind": "fe1TriggerKind",
			"body": "fe1Body",
			"attributes": {
				"fe1KeyA": "fe1StringValue",
				"fe1KeyB": ["fe1ListValueItemA", "fe1ListValueItemB"]
			}
		}
	},
	"fe2": {
		"metadata": {
			"name": "fe2",
			"namespace": "fe-namespace",
			"labels": {
				"nuclio.io/function-name": "feFunc"
			}
		},
		"spec": {
			"displayName": "fe2DisplayName",
			"triggerName": "fe2TriggerName",
			"triggerKind": "fe2TriggerKind",
			"body": "fe2Body",
			"attributes": {
				"fe2KeyA": "fe2StringValue",
				"fe2KeyB": ["fe2ListValueItemA", "fe2ListValueItemB"]
			}
		}
	}
}`

	suite.sendRequest("GET",
		"/api/function_events",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionEventTestSuite) TestGetListNoNamespace() {
	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Namespace must exist"})
	suite.sendRequest("GET",
		"/api/function_events",
		nil,
		nil,
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionEventTestSuite) TestCreateSuccessful() {

	// verify
	verifyCreateFunctionEvent := func(createFunctionEventOptions *platform.CreateFunctionEventOptions) bool {
		suite.Require().Equal("fe1", createFunctionEventOptions.FunctionEventConfig.Meta.Name)
		suite.Require().Equal("fe1-namespace", createFunctionEventOptions.FunctionEventConfig.Meta.Namespace)
		suite.Require().Equal("fe1Func", createFunctionEventOptions.FunctionEventConfig.Meta.Labels["nuclio.io/function-name"])
		suite.Require().Equal("fe1DisplayName", createFunctionEventOptions.FunctionEventConfig.Spec.DisplayName)
		suite.Require().Equal("fe1TriggerName", createFunctionEventOptions.FunctionEventConfig.Spec.TriggerName)
		suite.Require().Equal("fe1TriggerKind", createFunctionEventOptions.FunctionEventConfig.Spec.TriggerKind)
		suite.Require().Equal("fe1Body", createFunctionEventOptions.FunctionEventConfig.Spec.Body)
		suite.Require().Equal(map[string]interface{}{
			"fe1KeyA": "fe1StringValue",
			"fe1KeyB": []interface{}{"fe1ListValueItemA", "fe1ListValueItemB"},
		}, createFunctionEventOptions.FunctionEventConfig.Spec.Attributes)

		return true
	}

	suite.mockPlatform.
		On("CreateFunctionEvent", mock.Anything, mock.MatchedBy(verifyCreateFunctionEvent)).
		Return(nil).
		Once()

	expectedStatusCode := http.StatusCreated
	requestBody := `{
	"metadata": {
		"name": "fe1",
		"namespace": "fe1-namespace",
		"labels": {
			"nuclio.io/function-name": "fe1Func"
		}
	},
	"spec": {
		"displayName": "fe1DisplayName",
		"triggerName": "fe1TriggerName",
		"triggerKind": "fe1TriggerKind",
		"body": "fe1Body",
		"attributes": {
			"fe1KeyA": "fe1StringValue",
			"fe1KeyB": ["fe1ListValueItemA", "fe1ListValueItemB"]
		}
	}
}`

	suite.sendRequest("POST",
		"/api/function_events",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		requestBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionEventTestSuite) TestCreateNoMetadata() {
	suite.sendRequestNoMetadata("POST")
}

func (suite *functionEventTestSuite) TestCreateNoName() {
	suite.mockPlatform.
		On("CreateFunctionEvent", mock.Anything, mock.Anything).
		Return(nil).
		Once()

	expectedStatusCode := http.StatusCreated
	requestBody := `{
	"metadata": {
		"namespace": "fe1-namespace"
	},
	"spec": {
		"displayName": "fe1DisplayName",
		"triggerName": "fe1TriggerName",
		"triggerKind": "fe1TriggerKind",
		"body": "fe1Body",
		"attributes": {
			"fe1KeyA": "fe1StringValue",
			"fe1KeyB": ["fe1ListValueItemA", "fe1ListValueItemB"]
		}
	}
}`

	responseVerifier := func(response map[string]interface{}) bool {

		// get metadata as a map
		metadata := response["metadata"].(map[string]interface{})

		// get name
		name := metadata["name"].(string)

		// make sure that name was populated with a UUID
		_, err := uuid.FromString(name)

		suite.Require().NoError(err, "Name must contain UUID: %s", name)

		return true
	}

	suite.sendRequest("POST",
		"/api/function_events",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		responseVerifier)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionEventTestSuite) TestCreateNoNamespace() {
	suite.sendRequestNoNamespace("POST")
}

func (suite *functionEventTestSuite) TestUpdateSuccessful() {

	// verify
	verifyUpdateFunctionEvent := func(updateFunctionEventOptions *platform.UpdateFunctionEventOptions) bool {
		suite.Require().Equal("fe1", updateFunctionEventOptions.FunctionEventConfig.Meta.Name)
		suite.Require().Equal("fe1-namespace", updateFunctionEventOptions.FunctionEventConfig.Meta.Namespace)
		suite.Require().Equal("fe1Func", updateFunctionEventOptions.FunctionEventConfig.Meta.Labels["nuclio.io/function-name"])
		suite.Require().Equal("fe1DisplayName", updateFunctionEventOptions.FunctionEventConfig.Spec.DisplayName)
		suite.Require().Equal("fe1TriggerName", updateFunctionEventOptions.FunctionEventConfig.Spec.TriggerName)
		suite.Require().Equal("fe1TriggerKind", updateFunctionEventOptions.FunctionEventConfig.Spec.TriggerKind)
		suite.Require().Equal("fe1Body", updateFunctionEventOptions.FunctionEventConfig.Spec.Body)
		suite.Require().Equal(map[string]interface{}{
			"fe1KeyA": "fe1StringValue",
			"fe1KeyB": []interface{}{"fe1ListValueItemA", "fe1ListValueItemB"},
		}, updateFunctionEventOptions.FunctionEventConfig.Spec.Attributes)

		return true
	}

	suite.mockPlatform.
		On("UpdateFunctionEvent", mock.Anything, mock.MatchedBy(verifyUpdateFunctionEvent)).
		Return(nil).
		Once()

	expectedStatusCode := http.StatusNoContent
	requestBody := `{
	"metadata": {
		"name": "fe1",
		"namespace": "fe1-namespace",
		"labels": {
			"nuclio.io/function-name": "fe1Func"
		}
	},
	"spec": {
		"displayName": "fe1DisplayName",
		"triggerName": "fe1TriggerName",
		"triggerKind": "fe1TriggerKind",
		"body": "fe1Body",
		"attributes": {
			"fe1KeyA": "fe1StringValue",
			"fe1KeyB": ["fe1ListValueItemA", "fe1ListValueItemB"]
		}
	}
}`

	suite.sendRequest("PUT",
		"/api/function_events",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		nil)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionEventTestSuite) TestUpdateNoMetadata() {
	suite.sendRequestNoMetadata("PUT")
}

func (suite *functionEventTestSuite) TestUpdateNoName() {
	suite.sendRequestNoName("PUT")
}

func (suite *functionEventTestSuite) TestUpdateNoNamespace() {
	suite.sendRequestNoNamespace("PUT")
}

func (suite *functionEventTestSuite) TestDeleteSuccessful() {

	// verify
	verifyDeleteFunctionEvent := func(deleteFunctionEventOptions *platform.DeleteFunctionEventOptions) bool {
		suite.Require().Equal("fe1", deleteFunctionEventOptions.Meta.Name)
		suite.Require().Equal("fe1-namespace", deleteFunctionEventOptions.Meta.Namespace)

		return true
	}

	suite.mockPlatform.
		On("DeleteFunctionEvent", mock.Anything, mock.MatchedBy(verifyDeleteFunctionEvent)).
		Return(nil).
		Once()

	expectedStatusCode := http.StatusNoContent
	requestBody := `{
	"metadata": {
		"name": "fe1",
		"namespace": "fe1-namespace"
	}
}`

	suite.sendRequest("DELETE",
		"/api/function_events",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		nil)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *functionEventTestSuite) TestDeleteNoMetadata() {
	suite.sendRequestNoMetadata("DELETE")
}

func (suite *functionEventTestSuite) TestDeleteNoName() {
	suite.sendRequestNoName("DELETE")
}

func (suite *functionEventTestSuite) TestDeleteNoNamespace() {
	suite.sendRequestNoNamespace("DELETE")
}

func (suite *functionEventTestSuite) sendRequestNoMetadata(method string) {
	suite.sendRequestWithInvalidBody(method, `{
	"spec": {
		"triggerName": "tn",
		"triggerKind": "tk"
	}
}`)
}

func (suite *functionEventTestSuite) sendRequestNoNamespace(method string) {
	suite.sendRequestWithInvalidBody(method, `{
	"metadata": {
		"name": "name"
	},
	"spec": {
		"triggerName": "tn",
		"triggerKind": "tk"
	}
}`)
}

func (suite *functionEventTestSuite) sendRequestNoName(method string) {
	suite.sendRequestWithInvalidBody(method, `{
	"metadata": {
		"namespace": "namespace"
	},
	"spec": {
		"triggerName": "tn",
		"triggerKind": "tk"
	}
}`)
}

func (suite *functionEventTestSuite) sendRequestWithInvalidBody(method string, body string) {
	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Function event name must be provided in metadata"})
	requestBody := body

	suite.sendRequest(method,
		"/api/function_events",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

//
// Api Gateway
//

type apiGatewayTestSuite struct {
	dashboardTestSuite
}

func (suite *apiGatewayTestSuite) TestGetDetailSuccessful() {
	name := "agw1"
	namespace := "some-namespace"

	returnedAPIGateway := platform.AbstractAPIGateway{
		APIGatewayConfig: platform.APIGatewayConfig{
			Meta: platform.APIGatewayMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: platform.APIGatewaySpec{
				Host:        "some-host",
				Description: "some-desc",
				Path:        "some-path",
				Upstreams: []platform.APIGatewayUpstreamSpec{
					{
						Kind: platform.APIGatewayUpstreamKindNuclioFunction,
						NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
							Name: "f1",
						},
					}, {
						Kind: platform.APIGatewayUpstreamKindNuclioFunction,
						NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
							Name: "f2",
						},
						Percentage: 20,
					},
				},
				AuthenticationMode: ingress.AuthenticationModeBasicAuth,
				Authentication: &platform.APIGatewayAuthenticationSpec{
					BasicAuth: &platform.BasicAuth{
						Username: "user1",
						Password: "pass1",
					},
				},
			},
		},
	}

	// verify
	verifyGetAPIGateways := func(getAPIGatewaysOptions *platform.GetAPIGatewaysOptions) bool {
		suite.Require().Equal(name, getAPIGatewaysOptions.Name)
		suite.Require().Equal(namespace, getAPIGatewaysOptions.Namespace)

		return true
	}

	suite.mockPlatform.
		On("GetAPIGateways", mock.Anything, mock.MatchedBy(verifyGetAPIGateways)).
		Return([]platform.APIGateway{&returnedAPIGateway}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-api-gateway-namespace": namespace,
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
  "metadata": {
    "name": "agw1",
    "namespace": "some-namespace"
  },
  "spec": {
    "host": "some-host",
    "description": "some-desc",
    "path": "some-path",
    "authenticationMode": "basicAuth",
    "authentication": {
      "basicAuth": {
        "username": "user1",
        "password": "pass1"
      }
    },
    "upstreams": [
      {
        "kind": "nucliofunction",
        "nucliofunction": {
          "name": "f1"
        }
      },
      {
        "kind": "nucliofunction",
        "nucliofunction": {
          "name": "f2"
        },
        "percentage": 20
      }
    ]
  },
  "status": {}
}`

	suite.sendRequest("GET",
		"/api/api_gateways/agw1",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *apiGatewayTestSuite) TestGetDetailNoNamespace() {
	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Namespace must exist"})
	suite.sendRequest("GET",
		"/api/api_gateways/agw1",
		nil,
		nil,
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *apiGatewayTestSuite) TestGetListSuccessful() {
	namespace := "some-namespace"
	returnedAPIGateway1 := platform.AbstractAPIGateway{}
	returnedAPIGateway1.APIGatewayConfig.Meta.Name = "agw1"
	returnedAPIGateway1.APIGatewayConfig.Meta.Namespace = namespace

	returnedAPIGateway1.APIGatewayConfig.Spec.Name = "agw1"
	returnedAPIGateway1.APIGatewayConfig.Spec.Host = "some-host"
	returnedAPIGateway1.APIGatewayConfig.Spec.Description = "some-desc"
	returnedAPIGateway1.APIGatewayConfig.Spec.Path = "some-path"
	returnedAPIGateway1.APIGatewayConfig.Spec.Upstreams = []platform.APIGatewayUpstreamSpec{
		{
			Kind: platform.APIGatewayUpstreamKindNuclioFunction,
			NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
				Name: "f1",
			},
		}, {
			Kind: platform.APIGatewayUpstreamKindNuclioFunction,
			NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
				Name: "f2",
			},
			Percentage: 20,
		},
	}
	returnedAPIGateway1.APIGatewayConfig.Spec.AuthenticationMode = ingress.AuthenticationModeBasicAuth
	returnedAPIGateway1.APIGatewayConfig.Spec.Authentication = &platform.APIGatewayAuthenticationSpec{
		BasicAuth: &platform.BasicAuth{
			Username: "user1",
			Password: "pass1",
		},
	}

	returnedAPIGateway2 := platform.AbstractAPIGateway{}
	returnedAPIGateway2.APIGatewayConfig.Meta.Name = "agw2"
	returnedAPIGateway2.APIGatewayConfig.Meta.Namespace = namespace

	returnedAPIGateway2.APIGatewayConfig.Spec.Name = "agw2"
	returnedAPIGateway2.APIGatewayConfig.Spec.Host = "some-host2"
	returnedAPIGateway2.APIGatewayConfig.Spec.Description = "some-desc2"
	returnedAPIGateway2.APIGatewayConfig.Spec.Path = "some-path2"
	returnedAPIGateway2.APIGatewayConfig.Spec.Upstreams = []platform.APIGatewayUpstreamSpec{
		{
			Kind: platform.APIGatewayUpstreamKindNuclioFunction,
			NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
				Name: "f3",
			},
		}, {
			Kind: platform.APIGatewayUpstreamKindNuclioFunction,
			NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
				Name: "f4",
			},
			Percentage: 50,
		},
	}
	// verify
	verifyGetAPIGateways := func(getAPIGatewaysOptions *platform.GetAPIGatewaysOptions) bool {
		suite.Require().Equal("", getAPIGatewaysOptions.Name)
		suite.Require().Equal(namespace, getAPIGatewaysOptions.Namespace)

		return true
	}

	suite.mockPlatform.
		On("GetAPIGateways", mock.Anything, mock.MatchedBy(verifyGetAPIGateways)).
		Return([]platform.APIGateway{&returnedAPIGateway1, &returnedAPIGateway2}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-api-gateway-namespace": namespace,
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
  "agw1": {
    "metadata": {
      "name": "agw1",
      "namespace": "some-namespace"
    },
    "spec": {
      "host": "some-host",
      "name": "agw1",
      "description": "some-desc",
      "path": "some-path",
      "authenticationMode": "basicAuth",
      "authentication": {
        "basicAuth": {
          "username": "user1",
          "password": "pass1"
        }
      },
      "upstreams": [
        {
          "kind": "nucliofunction",
          "nucliofunction": {
            "name": "f1"
          }
        },
        {
          "kind": "nucliofunction",
          "nucliofunction": {
            "name": "f2"
          },
          "percentage": 20
        }
      ]
    },
    "status": {}
  },
  "agw2": {
    "metadata": {
      "name": "agw2",
      "namespace": "some-namespace"
    },
    "spec": {
      "host": "some-host2",
      "name": "agw2",
      "description": "some-desc2",
      "path": "some-path2",
      "upstreams": [
        {
          "kind": "nucliofunction",
          "nucliofunction": {
            "name": "f3"
          }
        },
        {
          "kind": "nucliofunction",
          "nucliofunction": {
            "name": "f4"
          },
          "percentage": 50
        }
      ]
    },
    "status": {}
  }
}`

	suite.sendRequest("GET",
		"/api/api_gateways",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *apiGatewayTestSuite) TestGetListNoNamespace() {
	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Namespace must exist"})
	suite.sendRequest("GET",
		"/api/api_gateways",
		nil,
		nil,
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *apiGatewayTestSuite) TestCreateSuccessful() {

	// verify
	verifyCreateAPIGateway := func(createAPIGatewayOptions *platform.CreateAPIGatewayOptions) bool {
		suite.Require().Equal("agw2", createAPIGatewayOptions.APIGatewayConfig.Meta.Name)
		suite.Require().Equal("some-namespace", createAPIGatewayOptions.APIGatewayConfig.Meta.Namespace)

		suite.Require().Equal("agw2", createAPIGatewayOptions.APIGatewayConfig.Spec.Name)
		suite.Require().Equal("some-host2", createAPIGatewayOptions.APIGatewayConfig.Spec.Host)
		suite.Require().Equal("some-desc2", createAPIGatewayOptions.APIGatewayConfig.Spec.Description)
		suite.Require().Equal("some-path2", createAPIGatewayOptions.APIGatewayConfig.Spec.Path)
		suite.Require().Equal(platform.APIGatewayUpstreamKindNuclioFunction, createAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[0].Kind)
		suite.Require().Equal("f3", createAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[0].NuclioFunction.Name)
		suite.Require().Equal(platform.APIGatewayUpstreamKindNuclioFunction, createAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[1].Kind)
		suite.Require().Equal("f4", createAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[1].NuclioFunction.Name)
		suite.Require().Equal(50, createAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[1].Percentage)

		return true
	}

	suite.mockPlatform.
		On("CreateAPIGateway", mock.Anything, mock.MatchedBy(verifyCreateAPIGateway)).
		Return(nil).
		Once()

	expectedStatusCode := http.StatusCreated
	requestBody := `{
    "metadata": {
      "name": "agw2",
      "namespace": "some-namespace"
    },
    "spec": {
      "host": "some-host2",
      "name": "agw2",
      "description": "some-desc2",
      "path": "some-path2",
      "upstreams": [
        {
          "kind": "nucliofunction",
          "nucliofunction": {
            "name": "f3"
          }
        },
        {
          "kind": "nucliofunction",
          "nucliofunction": {
            "name": "f4"
          },
          "percentage": 50
        }
      ]
    },
    "status": {}
  }`

	suite.sendRequest("POST",
		"/api/api_gateways",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		requestBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *apiGatewayTestSuite) TestUpdateSuccessful() {

	// verify
	verifyUpdateAPIGateway := func(updateAPIGatewayOptions *platform.UpdateAPIGatewayOptions) bool {
		suite.Require().Equal("agw2", updateAPIGatewayOptions.APIGatewayConfig.Meta.Name)
		suite.Require().Equal("some-namespace", updateAPIGatewayOptions.APIGatewayConfig.Meta.Namespace)

		suite.Require().Equal("agw2", updateAPIGatewayOptions.APIGatewayConfig.Spec.Name)
		suite.Require().Equal("some-host2", updateAPIGatewayOptions.APIGatewayConfig.Spec.Host)
		suite.Require().Equal("some-desc2", updateAPIGatewayOptions.APIGatewayConfig.Spec.Description)
		suite.Require().Equal("some-path2", updateAPIGatewayOptions.APIGatewayConfig.Spec.Path)
		suite.Require().Equal(platform.APIGatewayUpstreamKindNuclioFunction, updateAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[0].Kind)
		suite.Require().Equal("f3", updateAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[0].NuclioFunction.Name)
		suite.Require().Equal(platform.APIGatewayUpstreamKindNuclioFunction, updateAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[1].Kind)
		suite.Require().Equal("f4", updateAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[1].NuclioFunction.Name)
		suite.Require().Equal(50, updateAPIGatewayOptions.APIGatewayConfig.Spec.Upstreams[1].Percentage)

		return true
	}

	suite.mockPlatform.
		On("UpdateAPIGateway", mock.Anything, mock.MatchedBy(verifyUpdateAPIGateway)).
		Return(nil).
		Once()

	expectedStatusCode := http.StatusNoContent
	requestBody := `{
    "metadata": {
      "name": "agw2",
      "namespace": "some-namespace"
    },
    "spec": {
      "host": "some-host2",
      "name": "agw2",
      "description": "some-desc2",
      "path": "some-path2",
      "upstreams": [
        {
          "kind": "nucliofunction",
          "nucliofunction": {
            "name": "f3"
          }
        },
        {
          "kind": "nucliofunction",
          "nucliofunction": {
            "name": "f4"
          },
          "percentage": 50
        }
      ]
    },
    "status": {}
  }`

	suite.sendRequest("PUT",
		"/api/api_gateways",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		nil)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *apiGatewayTestSuite) TestDeleteSuccessful() {

	// verify
	verifyDeleteAPIGateway := func(deleteAPIGatewayOptions *platform.DeleteAPIGatewayOptions) bool {
		suite.Require().Equal("agw1", deleteAPIGatewayOptions.Meta.Name)
		suite.Require().Equal("some-namespace", deleteAPIGatewayOptions.Meta.Namespace)

		return true
	}

	suite.mockPlatform.
		On("DeleteAPIGateway", mock.Anything, mock.MatchedBy(verifyDeleteAPIGateway)).
		Return(nil).
		Once()

	expectedStatusCode := http.StatusNoContent
	requestBody := `{
	"metadata": {
		"name": "agw1",
		"namespace": "some-namespace"
	}
}`

	suite.sendRequest("DELETE",
		"/api/api_gateways",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		nil)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *apiGatewayTestSuite) sendRequestWithInvalidBody(method string, body string, expectedError string) {
	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{expectedError})
	requestBody := body

	suite.sendRequest(method,
		"/api/api_gateways",
		nil,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

//
// Misc
//

type miscTestSuite struct {
	dashboardTestSuite
}

func (suite *miscTestSuite) TestGetExternalIPAddresses() {
	returnedAddresses := []string{"address1", "address2", "address3"}

	suite.mockPlatform.
		On("GetExternalIPAddresses").
		Return(returnedAddresses, nil).
		Once()

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
	"externalIPAddresses": {
		"addresses": [
			"address1",
			"address2",
			"address3"
		]
	}
}`

	suite.sendRequest("GET",
		"/api/external_ip_addresses",
		nil,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *miscTestSuite) TestGetFrontendSpec() {
	returnedAddresses := []string{"address1", "address2", "address3"}
	imageNamePrefixTemplate := "{{ .ProjectName }}-{{ .FunctionName }}-"
	scaleToZeroConfiguration := platformconfig.ScaleToZero{
		Mode:                     platformconfig.EnabledScaleToZeroMode,
		ScalerInterval:           "",
		ResourceReadinessTimeout: "",
		ScaleResources: []functionconfig.ScaleResource{
			{
				MetricName: "metric_name",
				WindowSize: "1m",
				Threshold:  0,
			},
		},
		InactivityWindowPresets: []string{"1m", "2m"},
	}
	allowedAuthenticationModes := []string{string(ingress.AuthenticationModeNone),
		string(ingress.AuthenticationModeBasicAuth)}

	suite.mockPlatform.
		On("GetExternalIPAddresses").
		Return(returnedAddresses, nil).
		Once()

	suite.mockPlatform.
		On("GetImageNamePrefixTemplate").
		Return(imageNamePrefixTemplate, nil).
		Once()

	suite.mockPlatform.
		On("GetScaleToZeroConfiguration").
		Return(&scaleToZeroConfiguration).
		Once()

	suite.mockPlatform.
		On("GetAllowedAuthenticationModes").
		Return(allowedAuthenticationModes).
		Once()

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
    "defaultFunctionConfig": {
        "attributes": {
            "metadata": {},
            "spec": {
                "resources": {},
                "minReplicas": 1,
                "maxReplicas": 1,
                "targetCPU": 75,
                "triggers": {
                    "cron": {
                        "class": "",
                        "kind": "",
                        "name": "",
                        "workerAvailabilityTimeoutMilliseconds": 10000
                    },
                    "default-http": {
                        "class": "",
                        "kind": "http",
                        "name": "default-http",
                        "maxWorkers": 1,
                        "workerAvailabilityTimeoutMilliseconds": 10000,
                        "attributes": {
                            "serviceType": "NodePort"
                        }
                    },
                    "http": {
                        "class": "",
                        "kind": "",
                        "name": "",
                        "workerAvailabilityTimeoutMilliseconds": 10000,
                        "attributes": {
                            "serviceType": "NodePort"
                        }
                    }
                },
                "build": {},
                "platform": {},
                "readinessTimeoutSeconds": 60,
                "eventTimeout": ""
            }
        }
    },
    "defaultHTTPIngressHostTemplate": "{{ .FunctionName }}.{{ .ProjectName }}.{{ .Namespace }}.test.com",
    "externalIPAddresses": [
        "address1",
        "address2",
        "address3"
    ],
    "imageNamePrefixTemplate": "{{ .ProjectName }}-{{ .FunctionName }}-",
    "namespace": "",
    "scaleToZero": {
        "inactivityWindowPresets": [
            "1m",
            "2m"
        ],
        "mode": "enabled",
        "scaleResources": [
            {
                "metricName": "metric_name",
                "windowSize": "1m",
                "threshold": 0
            }
        ]
    },
	"validFunctionPriorityClassNames": null,
	"platformKind": "",
	"allowedAuthenticationModes": [
		"none",
		"basicAuth"
	]
}`

	suite.sendRequest("GET",
		"/api/frontend_spec",
		nil,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func TestDashboardTestSuite(t *testing.T) {
	suite.Run(t, new(functionTestSuite))
	suite.Run(t, new(projectTestSuite))
	suite.Run(t, new(functionEventTestSuite))
	suite.Run(t, new(apiGatewayTestSuite))
	suite.Run(t, new(miscTestSuite))
}
