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

	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
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

func (mfdc *MockFunctiondepClient) CreateOrUpdate(function *functioncr.Function) (*v1beta1.Deployment, error) {
	args := mfdc.Called(function)
	return args.Get(0).(*v1beta1.Deployment), args.Error(1)
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
	logger                nuclio.Logger
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

	// verify that fields were updated on function cr
	verifyUpdatedFunctioncr := func(f *functioncr.Function) bool {
		suite.Require().Equal("func-name", f.GetLabels()["name"])
		suite.Require().Equal("latest", f.GetLabels()["version"])
		suite.Require().Equal(-1, f.Spec.Version)
		suite.Require().Equal("latest", f.Spec.Alias)
		suite.Require().Equal(functioncr.FunctionStateProcessed, f.Status.State)

		return true
	}

	// expect update to happen on cr
	suite.mockFunctioncrClient.
		On("Update", mock.MatchedBy(verifyUpdatedFunctioncr)).
		Return(&function, nil).
		Once()

	// expect resource version to be ignored
	suite.mockChangeIgnorer.
		On("Push", "funcnamespace.func-name", "123").
		Once()

	// expect a function deployment to be created
	suite.mockFunctiondepClient.
		On("CreateOrUpdate", mock.MatchedBy(verifyUpdatedFunctioncr)).
		Return(&v1beta1.Deployment{}, nil).
		Once()

	err := suite.controller.addFunction(&function)
	suite.Require().NoError(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerCreateTestSuite) TestCreateLatestWithPublish() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Namespace = "funcnamespace"
	function.ResourceVersion = "123"
	function.Spec.Publish = true
	function.Spec.Runtime = "golang"
	function.Spec.Handler = "handler"

	//
	// Expect published function to be created
	//

	verifyPublishedFunctioncr := func(f *functioncr.Function) bool {
		suite.Require().Equal(suite.getVersionedName("funcname", "0"), f.Name)
		suite.Require().False(f.Spec.Publish)
		suite.Require().Equal("", f.ResourceVersion)
		suite.Require().Equal("", f.Spec.Alias)
		suite.Require().Equal("funcname", function.GetLabels()["name"])
		suite.Require().Equal("0", f.GetLabels()["version"])
		suite.Require().Equal(functioncr.FunctionStateProcessed, f.Status.State)

		return true
	}

	publishedFunction := function
	publishedFunction.Name = suite.getVersionedName("funcname", "0")
	publishedFunction.ResourceVersion = "555"

	// expect a function cr to be created. return a publishedFunction so we can test it is ignored
	suite.mockFunctioncrClient.
		On("Create", mock.MatchedBy(verifyPublishedFunctioncr)).
		Return(&publishedFunction, nil).
		Once()

	// expect created function
	suite.mockChangeIgnorer.
		On("Push", suite.getVersionedName("funcnamespace.funcname", "0"), "555").
		Once()

	// expect a function deployment to be created
	suite.mockFunctiondepClient.
		On("CreateOrUpdate", mock.Anything).
		Return(&v1beta1.Deployment{}, nil).
		Once()

	//
	// Expect latest function to be created
	//

	// verify that fields were updated on function cr
	verifyUpdatedFunctioncr := func(f *functioncr.Function) bool {
		suite.Require().Equal("funcname", f.GetLabels()["name"])
		suite.Require().Equal("latest", f.GetLabels()["version"])
		suite.Require().Equal(0, f.Spec.Version)
		suite.Require().Equal("latest", f.Spec.Alias)
		suite.Require().Equal(functioncr.FunctionStateProcessed, f.Status.State)
		suite.Require().Equal("123", f.ResourceVersion)
		suite.Require().False(f.Spec.Publish)

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

	// expect a function deployment to be created
	suite.mockFunctiondepClient.
		On("CreateOrUpdate", mock.MatchedBy(verifyUpdatedFunctioncr)).
		Return(&v1beta1.Deployment{}, nil).
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
		suite.Require().Equal(functioncr.FunctionStateError, f.Status.State)
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

func (suite *ControllerCreateTestSuite) TestUpdateErrorFunctionUpdated() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Namespace = "funcnamespace"
	function.ResourceVersion = "123"
	function.Spec.Version = 3

	// verify that fields were updated on function cr
	verifyUpdatedFunctioncr := func(f *functioncr.Function) bool {
		suite.Require().Equal(functioncr.FunctionStateError, f.Status.State)
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

	err := suite.controller.handleFunctionCRUpdate(&function)
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

func (suite *ControllerCreateTestSuite) TestCreateLatestInvalidVersionInName() {
	function := functioncr.Function{}
	function.Name = suite.getVersionedName("funcname", "30")

	err := suite.controller.addFunction(&function)
	suite.Require().Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerCreateTestSuite) TestCreateLatestInvalidAlias() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Spec.Alias = "wrong"

	err := suite.controller.addFunction(&function)
	suite.Require().Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

//
// Update
//

type ControllerUpdateTestSuite struct {
	ControllerTestSuite
}

func (suite *ControllerUpdateTestSuite) TestUpdateLatestPublish() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Namespace = "funcnamespace"
	function.ResourceVersion = "123"
	function.Spec.Alias = "latest"
	function.Spec.Publish = true
	function.Spec.Version = 2
	function.Spec.HTTPPort = 1111
	function.ObjectMeta.Labels = map[string]string{
		"name":    "funcname",
		"version": "latest",
	}

	//
	// Expect published function to be created
	//

	verifyPublishedFunctioncr := func(f *functioncr.Function) bool {
		suite.Require().Equal(suite.getVersionedName("funcname", "3"), f.Name)
		suite.Require().False(f.Spec.Publish)
		suite.Require().Equal("", f.ResourceVersion)
		suite.Require().Equal("", f.Spec.Alias)
		suite.Require().Equal("funcname", function.GetLabels()["name"])
		suite.Require().Equal("3", f.GetLabels()["version"])
		suite.Require().Equal(functioncr.FunctionStateProcessed, f.Status.State)
		suite.Require().Equal(1111, int(f.Spec.HTTPPort))

		return true
	}

	publishedFunction := function
	publishedFunction.Name = suite.getVersionedName("funcname", "3")
	publishedFunction.ResourceVersion = "555"

	// expect a function cr to be created. return a publishedFunction so we can test it is ignored
	suite.mockFunctioncrClient.
		On("Create", mock.MatchedBy(verifyPublishedFunctioncr)).
		Return(&publishedFunction, nil).
		Once()

	// expect created function
	suite.mockChangeIgnorer.
		On("Push", suite.getVersionedName("funcnamespace.funcname", "3"), "555").
		Once()

	// expect a function deployment to be created
	suite.mockFunctiondepClient.
		On("CreateOrUpdate", mock.Anything).
		Return(&v1beta1.Deployment{}, nil).
		Once()

	//
	// Expect latest function to be created
	//

	// verify that fields were updated on function cr
	verifyUpdatedFunctioncr := func(f *functioncr.Function) bool {
		suite.Require().Equal("funcname", f.GetLabels()["name"])
		suite.Require().Equal("latest", f.GetLabels()["version"])
		suite.Require().Equal(3, f.Spec.Version)
		suite.Require().Equal("latest", f.Spec.Alias)
		suite.Require().Equal(functioncr.FunctionStateProcessed, f.Status.State)
		suite.Require().Equal("123", f.ResourceVersion)
		suite.Require().False(f.Spec.Publish)
		suite.Require().Equal(1111, int(f.Spec.HTTPPort))

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

	// expect a function deployment to be updated
	suite.mockFunctiondepClient.
		On("CreateOrUpdate", mock.MatchedBy(verifyUpdatedFunctioncr)).
		Return(&v1beta1.Deployment{}, nil).
		Once()

	err := suite.controller.updateFunction(&function)
	suite.Require().NoError(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerUpdateTestSuite) TestUpdatePublished() {
	function := functioncr.Function{}
	function.Name = suite.getVersionedName("funcname", "2")
	function.Namespace = "funcnamespace"
	function.ResourceVersion = "123"
	function.Spec.Version = 2
	function.Spec.HTTPPort = 1111

	// verify that fields were updated on function cr
	verifyUpdatedFunctioncr := func(f *functioncr.Function) bool {
		suite.Require().Equal(2, f.Spec.Version)
		suite.Require().Equal(functioncr.FunctionStateProcessed, f.Status.State)
		suite.Require().Equal("123", f.ResourceVersion)
		suite.Require().False(f.Spec.Publish)
		suite.Require().Equal(1111, int(f.Spec.HTTPPort))

		return true
	}

	// expect update to happen on cr
	suite.mockFunctioncrClient.
		On("Update", mock.MatchedBy(verifyUpdatedFunctioncr)).
		Return(&function, nil).
		Once()

	// expect resource version to be ignored
	suite.mockChangeIgnorer.
		On("Push", suite.getVersionedName("funcnamespace.funcname", "2"), "123").
		Once()

	// expect a function deployment to be updated
	suite.mockFunctiondepClient.
		On("CreateOrUpdate", mock.MatchedBy(verifyUpdatedFunctioncr)).
		Return(&v1beta1.Deployment{}, nil).
		Once()

	err := suite.controller.updateFunction(&function)
	suite.Require().NoError(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerUpdateTestSuite) TestUpdatePublishedInvalidRepublish() {
	function := functioncr.Function{}
	function.Name = suite.getVersionedName("funcname", "2")
	function.Namespace = "funcnamespace"
	function.Spec.Publish = true

	err := suite.controller.updateFunction(&function)
	suite.Require().Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerCreateTestSuite))
	suite.Run(t, new(ControllerUpdateTestSuite))
}
