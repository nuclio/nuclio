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

package app

import (
	"fmt"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	v1beta1 "k8s.io/api/apps/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//
// Custom resource client mock
//

type MockFunctioncrClient struct {
	mock.Mock
}

func (mfcc *MockFunctioncrClient) CreateResource() error {
	return nil
}

func (mfcc *MockFunctioncrClient) DeleteResource() error {
	return nil
}

func (mfcc *MockFunctioncrClient) WaitForResource() error {
	return nil
}

func (mfcc *MockFunctioncrClient) WatchForChanges(namespace string, changeChan chan functioncr.Change) (*functioncr.Watcher, error) {
	return nil, nil
}

func (mfcc *MockFunctioncrClient) Create(function *functioncr.Function) (*functioncr.Function, error) {
	args := mfcc.Called(function)
	return args.Get(0).(*functioncr.Function), args.Error(1)
}

func (mfcc *MockFunctioncrClient) Update(function *functioncr.Function) (*functioncr.Function, error) {
	args := mfcc.Called(function)

	return args.Get(0).(*functioncr.Function), args.Error(1)
}

func (mfcc *MockFunctioncrClient) Delete(namespace, name string, options *meta_v1.DeleteOptions) error {
	args := mfcc.Called(namespace, name, options)
	return args.Error(0)
}

func (mfcc *MockFunctioncrClient) Get(namespace, name string) (*functioncr.Function, error) {
	args := mfcc.Called(namespace, name)
	return args.Get(0).(*functioncr.Function), args.Error(1)
}

func (mfcc *MockFunctioncrClient) List(namespace string, options *meta_v1.ListOptions) (*functioncr.FunctionList, error) {
	args := mfcc.Called(namespace, options)
	return args.Get(0).(*functioncr.FunctionList), args.Error(1)
}

//
// Deployed function client mock
//

type MockFunctiondepClient struct {
	mock.Mock
}

func (mfdc *MockFunctiondepClient) List(namespace string) ([]v1beta1.Deployment, error) {
	args := mfdc.Called(namespace)
	return args.Get(0).([]v1beta1.Deployment), args.Error(1)
}

func (mfdc *MockFunctiondepClient) Get(namespace string, name string) (*v1beta1.Deployment, error) {
	args := mfdc.Called(namespace, name)
	return args.Get(0).(*v1beta1.Deployment), args.Error(1)
}

func (mfdc *MockFunctiondepClient) CreateOrUpdate(function *functioncr.Function, imagePullSecrets string) (*v1beta1.Deployment, error) {
	args := mfdc.Called(function)
	return args.Get(0).(*v1beta1.Deployment), args.Error(1)
}

func (mfdc *MockFunctiondepClient) WaitAvailable(namespace string, name string) error {
	args := mfdc.Called(namespace, name)
	return args.Error(0)
}

func (mfdc *MockFunctiondepClient) Delete(namespace string, name string) error {
	args := mfdc.Called(namespace, name)
	return args.Error(0)
}

//
// Change ignorer mock
//

type MockChangeIgnorer struct {
	mock.Mock
}

func (mci *MockChangeIgnorer) Push(namespacedName string, resourceVersion string) {
	mci.Called(namespacedName, resourceVersion)
}

func (mci *MockChangeIgnorer) Pop(namespacedName string, resourceVersion string) bool {
	args := mci.Called(namespacedName, resourceVersion)
	return args.Bool(0)
}

//
// Test suite
//

type ControllerTestSuite struct {
	suite.Suite
	logger                logger.Logger
	controller            Controller
	mockFunctioncrClient  *MockFunctioncrClient
	mockFunctiondepClient *MockFunctiondepClient
	mockChangeIgnorer     *MockChangeIgnorer
}

func (suite *ControllerTestSuite) SetupTest() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")

	suite.mockFunctioncrClient = &MockFunctioncrClient{}
	suite.mockFunctiondepClient = &MockFunctiondepClient{}
	suite.mockChangeIgnorer = &MockChangeIgnorer{}

	// manually create a controller
	suite.controller = Controller{
		logger:                   suite.logger,
		functioncrClient:         suite.mockFunctioncrClient,
		functiondepClient:        suite.mockFunctiondepClient,
		ignoredFunctionCRChanges: suite.mockChangeIgnorer,
	}
}

func (suite *ControllerTestSuite) getVersionedName(name string, version string) string {
	return fmt.Sprintf("%s%s%s", name, functioncr.GetVersionSeparator(), version)
}

//
// Create
//

type ControllerCreateTestSuite struct {
	ControllerTestSuite
}

func (suite *ControllerCreateTestSuite) TestCreate() {
	function := functioncr.Function{}
	function.Name = "func-name"
	function.Namespace = "funcnamespace"
	function.ResourceVersion = "123"
	function.Spec.Runtime = "golang"
	function.Spec.Handler = "handler"
	function.Status.State = functionconfig.FunctionStateNotReady

	// verify that fields were updated on function cr
	verifyFunctioncr := func(f *functioncr.Function) bool {
		suite.Require().Equal(functionconfig.FunctionStateNotReady, f.Status.State)

		return true
	}

	// verify that fields were updated on function cr
	verifyUpdatedFunctioncr := func(f *functioncr.Function) bool {
		suite.Require().Equal(functionconfig.FunctionStateReady, f.Status.State)

		return true
	}

	// expect a function deployment to be created
	suite.mockFunctiondepClient.
		On("CreateOrUpdate", mock.MatchedBy(verifyFunctioncr)).
		Return(&v1beta1.Deployment{}, nil).
		Once()

	// expect the deployment's availability to be waited on
	suite.mockFunctiondepClient.
		On("WaitAvailable", mock.Anything, mock.Anything).
		Return(nil).
		Once()

	// expect update to happen on cr
	suite.mockFunctioncrClient.
		On("Update", mock.MatchedBy(verifyUpdatedFunctioncr)).
		Return(&function, nil).
		Once()

	// expect resource version to be ignored
	suite.mockChangeIgnorer.
		On("Push", "funcnamespace.func-name", "123").
		Once()

	err := suite.controller.addFunction(&function)
	suite.Require().NoError(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerCreateTestSuite) TestCreateErrorFunctionUpdated() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Namespace = "funcnamespace"
	function.ResourceVersion = "123"
	function.Spec.Version = 3

	// verify that fields were updated on function cr
	verifyUpdatedFunctioncr := func(f *functioncr.Function) bool {
		suite.Require().Equal(functionconfig.FunctionStateError, f.Status.State)
		suite.Require().Equal("Validation failed", f.Status.Message)

		return true
	}

	// expect update to happen on cr
	suite.mockFunctioncrClient.
		On("Update", mock.MatchedBy(verifyUpdatedFunctioncr)).
		Return(&function, nil).
		Once()

	// expect resource version to be ignored
	suite.mockChangeIgnorer.
		On("Push", "funcnamespace.funcname", "123").
		Once()

	err := suite.controller.handleFunctionCRAdd(&function)
	suite.Require().Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerCreateTestSuite) TestCreateStatusAndMessageSet() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Spec.Alias = "wrong"

	err := suite.controller.addFunction(&function)
	suite.Require().Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerCreateTestSuite) TestCreateLatestInvalidVersionInSpec() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Spec.Version = 50

	err := suite.controller.addFunction(&function)
	suite.Require().Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerCreateTestSuite))
}
