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
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nuclio/nuclio/pkg/dashboard"
	_ "github.com/nuclio/nuclio/pkg/dashboard/resource"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/restful"
	"github.com/nuclio/nuclio/test/compare"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

//
// Platform mock
//

// Platform defines the interface that any underlying function platform must provide for nuclio
// to run over it
type mockPlatform struct {
	mock.Mock
}

// Build will locally build a processor image and return its name (or the error)
func (mp *mockPlatform) BuildFunction(buildOptions *platform.BuildOptions) (*platform.BuildResult, error) {
	args := mp.Called(buildOptions)
	return args.Get(0).(*platform.BuildResult), args.Error(1)
}

// Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
func (mp *mockPlatform) DeployFunction(deployOptions *platform.DeployOptions) (*platform.DeployResult, error) {
	args := mp.Called(deployOptions)
	return args.Get(0).(*platform.DeployResult), args.Error(1)
}

// UpdateOptions will update a previously deployed function
func (mp *mockPlatform) UpdateFunction(updateOptions *platform.UpdateOptions) error {
	args := mp.Called(updateOptions)
	return args.Error(0)
}

// DeleteFunction will delete a previously deployed function
func (mp *mockPlatform) DeleteFunction(deleteOptions *platform.DeleteOptions) error {
	args := mp.Called(deleteOptions)
	return args.Error(0)
}

// InvokeFunction will invoke a previously deployed function
func (mp *mockPlatform) InvokeFunction(invokeOptions *platform.InvokeOptions) (*platform.InvokeResult, error) {
	args := mp.Called(invokeOptions)
	return args.Get(0).(*platform.InvokeResult), args.Error(1)
}

// InvokeFunction will invoke a previously deployed function
func (mp *mockPlatform) GetFunctions(getOptions *platform.GetOptions) ([]platform.Function, error) {
	args := mp.Called(getOptions)
	return args.Get(0).([]platform.Function), args.Error(1)
}

// GetDeployRequiresRegistry returns true if a registry is required for deploy, false otherwise
func (mp *mockPlatform) GetDeployRequiresRegistry() bool {
	args := mp.Called()
	return args.Bool(0)
}

// GetName returns the platform name
func (mp *mockPlatform) GetName() string {
	args := mp.Called()
	return args.String(0)
}

// GetNodes returns a slice of nodes currently in the cluster
func (mp *mockPlatform) GetNodes() ([]platform.Node, error) {
	args := mp.Called()
	return args.Get(0).([]platform.Node), args.Error(1)
}

//
// Test suite
//

type dashboardTestSuite struct {
	suite.Suite
	logger          logger.Logger
	dashboardServer *dashboard.Server
	httpServer      *httptest.Server
	mockPlatform    *mockPlatform
}

func (suite *dashboardTestSuite) SetupTest() {
	var err error

	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
	suite.mockPlatform = &mockPlatform{}

	// create a mock platform
	suite.dashboardServer, err = dashboard.NewServer(suite.logger,
		"",
		"",
		"",
		"",
		suite.mockPlatform,
		true,
		&platformconfig.WebServer{},
		nil)

	if err != nil {
		panic("Failed to create server")
	}

	// create an http server from the dashboard server
	suite.httpServer = httptest.NewServer(suite.dashboardServer.Router)
}

func (suite *dashboardTestSuite) TeardownTest() {
	suite.httpServer.Close()
}

func (suite *dashboardTestSuite) TestGetDetailSuccessful() {
	returnedFunction := platform.AbstractFunction{}
	returnedFunction.Config.Meta.Name = "f1"
	returnedFunction.Config.Meta.Namespace = "f1Namespace"
	returnedFunction.Config.Spec.Replicas = 10

	// verify
	verifyGetFunctions := func(getOptions *platform.GetOptions) bool {
		suite.Require().Equal("f1", getOptions.Name)
		suite.Require().Equal("f1Namespace", getOptions.Namespace)

		return true
	}

	suite.mockPlatform.
		On("GetFunctions", mock.MatchedBy(verifyGetFunctions)).
		Return([]platform.Function{&returnedFunction}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-function-namespace": "f1Namespace",
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
	"metadata": {
		"name": "f1",
		"namespace": "f1Namespace"
	},
	"spec": {
		"resources": {},
		"build": {},
		"replicas": 10
	}
}`

	suite.sendRequest("GET",
		"/functions/f1",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *dashboardTestSuite) TestGetDetailNoNamespace() {
	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Namespace must exist"})
	suite.sendRequest("GET",
		"/functions/f1",
		nil,
		nil,
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *dashboardTestSuite) TestGetListSuccessful() {
	returnedFunction1 := platform.AbstractFunction{}
	returnedFunction1.Config.Meta.Name = "f1"
	returnedFunction1.Config.Meta.Namespace = "fNamespace"
	returnedFunction1.Config.Spec.Runtime = "r1"

	returnedFunction2 := platform.AbstractFunction{}
	returnedFunction2.Config.Meta.Name = "f2"
	returnedFunction2.Config.Meta.Namespace = "fNamespace"
	returnedFunction2.Config.Spec.Runtime = "r2"

	// verify
	verifyGetFunctions := func(getOptions *platform.GetOptions) bool {
		suite.Require().Equal("", getOptions.Name)
		suite.Require().Equal("fNamespace", getOptions.Namespace)

		return true
	}

	suite.mockPlatform.
		On("GetFunctions", mock.MatchedBy(verifyGetFunctions)).
		Return([]platform.Function{&returnedFunction1, &returnedFunction2}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-function-namespace": "fNamespace",
	}

	expectedStatusCode := http.StatusOK
	expectedResponseBody := `{
	"f1": {
		"metadata": {
			"name": "f1",
			"namespace": "fNamespace"
		},
		"spec": {
			"resources": {},
			"build": {},
			"runtime": "r1"
		}
	},
	"f2": {
		"metadata": {
			"name": "f2",
			"namespace": "fNamespace"
		},
		"spec": {
			"resources": {},
			"build": {},
			"runtime": "r2"
		}
	}
}`

	suite.sendRequest("GET",
		"/functions",
		headers,
		nil,
		&expectedStatusCode,
		expectedResponseBody)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *dashboardTestSuite) TestGetListNoNamespace() {
	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Namespace must exist"})
	suite.sendRequest("GET",
		"/functions",
		nil,
		nil,
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *dashboardTestSuite) TestCreateSuccessful() {

	// verify
	verifyDeployFunction := func(deployOptions *platform.DeployOptions) bool {
		suite.Require().Equal("f1", deployOptions.FunctionConfig.Meta.Name)
		suite.Require().Equal("f1Namespace", deployOptions.FunctionConfig.Meta.Namespace)

		return true
	}

	suite.mockPlatform.
		On("DeployFunction", mock.MatchedBy(verifyDeployFunction)).
		Return(&platform.DeployResult{}, nil).
		Once()

	headers := map[string]string{
		"x-nuclio-wait-function-action": "true",
	}

	expectedStatusCode := http.StatusAccepted
	requestBody := `{
	"metadata": {
		"name": "f1",
		"namespace": "f1Namespace"
	},
	"spec": {
		"resources": {},
		"build": {},
		"runtime": "r1"
	}
}`

	suite.sendRequest("POST",
		"/functions",
		headers,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		nil)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *dashboardTestSuite) TestCreateNoMetadata() {
	suite.sendRequestNoMetadata("POST")
}

func (suite *dashboardTestSuite) TestCreateNoName() {
	suite.sendRequestNoName("POST")
}

func (suite *dashboardTestSuite) TestCreateNoNamespace() {
	suite.sendRequestNoNamespace("POST")
}

func (suite *dashboardTestSuite) TestUpdateSuccessful() {

	// verify
	verifyUpdateFunction := func(updateOptions *platform.UpdateOptions) bool {
		suite.Require().Equal("f1", updateOptions.FunctionMeta.Name)
		suite.Require().Equal("f1Namespace", updateOptions.FunctionMeta.Namespace)

		return true
	}

	suite.mockPlatform.
		On("UpdateFunction", mock.MatchedBy(verifyUpdateFunction)).
		Return(nil).
		Once()

	headers := map[string]string{
		"x-nuclio-wait-function-action": "true",
	}

	expectedStatusCode := http.StatusAccepted
	requestBody := `{
	"metadata": {
		"name": "f1",
		"namespace": "f1Namespace"
	},
	"spec": {
		"resources": {},
		"build": {},
		"runtime": "r1"
	}
}`

	suite.sendRequest("PUT",
		"/functions",
		headers,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		nil)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *dashboardTestSuite) TestUpdateNoMetadata() {
	suite.sendRequestNoMetadata("PUT")
}

func (suite *dashboardTestSuite) TestUpdateNoName() {
	suite.sendRequestNoName("PUT")
}

func (suite *dashboardTestSuite) TestUpdateNoNamespace() {
	suite.sendRequestNoNamespace("PUT")
}

func (suite *dashboardTestSuite) TestDeleteSuccessful() {

	// verify
	verifyDeleteFunction := func(deleteOptions *platform.DeleteOptions) bool {
		suite.Require().Equal("f1", deleteOptions.FunctionConfig.Meta.Name)
		suite.Require().Equal("f1Namespace", deleteOptions.FunctionConfig.Meta.Namespace)

		return true
	}

	suite.mockPlatform.
		On("DeleteFunction", mock.MatchedBy(verifyDeleteFunction)).
		Return(nil).
		Once()

	headers := map[string]string{
		"x-nuclio-wait-function-action": "true",
	}

	expectedStatusCode := http.StatusNoContent
	requestBody := `{
	"metadata": {
		"name": "f1",
		"namespace": "f1Namespace"
	}
}`

	suite.sendRequest("DELETE",
		"/functions",
		headers,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		nil)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func (suite *dashboardTestSuite) TestDeleteNoMetadata() {
	suite.sendRequestNoMetadata("DELETE")
}

func (suite *dashboardTestSuite) TestDeleteNoName() {
	suite.sendRequestNoName("DELETE")
}

func (suite *dashboardTestSuite) TestDeleteNoNamespace() {
	suite.sendRequestNoNamespace("DELETE")
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

	defer response.Body.Close()

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

		err = json.Unmarshal(encodedResponseBody, &decodedResponseBody)
		suite.Require().NoError(err)

		suite.logger.DebugWith("Comparing expected", "expected", encodedExpectedResponse)

		switch typedEncodedExpectedResponse := encodedExpectedResponse.(type) {
		case string:
			decodedExpectedResponseBody := map[string]interface{}{}

			err = json.Unmarshal([]byte(typedEncodedExpectedResponse), &decodedExpectedResponseBody)
			suite.Require().NoError(err)

			suite.Require().True(compare.CompareNoOrder(decodedExpectedResponseBody, decodedResponseBody))

		case func(response map[string]interface{}) bool:
			suite.Require().True(typedEncodedExpectedResponse(decodedResponseBody))

		default:
			panic("Unsupported expected response verifier")
		}
	}

	return response, decodedResponseBody
}

func (suite *dashboardTestSuite) sendRequestNoMetadata(method string) {
	suite.sendRequestWithInvalidBody(method, `{
	"spec": {
		"resources": {},
		"build": {},
		"runtime": "r1"
	}
}`)
}

func (suite *dashboardTestSuite) sendRequestNoNamespace(method string) {
	suite.sendRequestWithInvalidBody(method, `{
	"metadata": {
		"namespace": "f1Namespace"
	},
	"spec": {
		"resources": {},
		"build": {},
		"runtime": "r1"
	}
}`)
}

func (suite *dashboardTestSuite) sendRequestNoName(method string) {
	suite.sendRequestWithInvalidBody(method, `{
	"metadata": {
		"namespace": "f1Namespace"
	},
	"spec": {
		"resources": {},
		"build": {},
		"runtime": "r1"
	}
}`)
}

func (suite *dashboardTestSuite) sendRequestWithInvalidBody(method string, body string) {
	headers := map[string]string{
		"x-nuclio-wait-function-action": "true",
	}

	expectedStatusCode := http.StatusBadRequest
	ecv := restful.NewErrorContainsVerifier(suite.logger, []string{"Function name and namespace must be provided in metadata"})
	requestBody := body

	suite.sendRequest(method,
		"/functions",
		headers,
		bytes.NewBufferString(requestBody),
		&expectedStatusCode,
		ecv.Verify)

	suite.mockPlatform.AssertExpectations(suite.T())
}

func TestDashboardTestSuite(t *testing.T) {
	suite.Run(t, new(dashboardTestSuite))
}
