package app

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/functioncr"
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
	suite.logger, _ = nucliozap.NewNuclioZap("test", nucliozap.DebugLevel)

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

//
// Create
//

type ControllerCreateTestSuite struct {
	ControllerTestSuite
}

func (suite *ControllerCreateTestSuite) TestLatestCreateSuccessful() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Namespace = "funcnamespace"
	function.ResourceVersion = "123"

	// verify that fields were updated on function cr
	verifyUpdatedFunctioncr := func(f *functioncr.Function) bool {
		suite.Equal("funcname", f.GetLabels()["name"])
		suite.Equal("latest", f.GetLabels()["version"])
		suite.Equal(-1, f.Spec.Version)
		suite.Equal("latest", f.Spec.Alias)
		suite.Equal(functioncr.FunctionStateProcessed, f.Status.State)

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
	suite.NoError(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerCreateTestSuite) TestLatestCreateAndPublish() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Namespace = "funcnamespace"
	function.ResourceVersion = "123"
	function.Spec.Publish = true

	//
	// Expect published function to be created
	//

	verifyPublishedFunctioncr := func(f *functioncr.Function) bool {
		suite.Equal("funcname-0", f.Name)
		suite.False(f.Spec.Publish)
		suite.Equal("", f.ResourceVersion)
		suite.Equal("", f.Spec.Alias)
		suite.Equal("funcname", function.GetLabels()["name"])
		suite.Equal("0", f.GetLabels()["version"])
		suite.Equal(functioncr.FunctionStateProcessed, f.Status.State)

		return true
	}

	publishedFunction := function
	publishedFunction.Name = "funcname-0"
	publishedFunction.ResourceVersion = "555"

	// expect a function cr to be created. return a publishedFunction so we can test it is ignored
	suite.mockFunctioncrClient.
		On("Create", mock.MatchedBy(verifyPublishedFunctioncr)).
		Return(&publishedFunction, nil).
		Once()

	// expect created function
	suite.mockChangeIgnorer.
		On("Push", "funcnamespace.funcname-0", "555").
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
		suite.Equal("funcname", f.GetLabels()["name"])
		suite.Equal("latest", f.GetLabels()["version"])
		suite.Equal(0, f.Spec.Version)
		suite.Equal("latest", f.Spec.Alias)
		suite.Equal(functioncr.FunctionStateProcessed, f.Status.State)
		suite.Equal("123", f.ResourceVersion)
		suite.False(f.Spec.Publish)

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
	suite.NoError(err)

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
		suite.Equal(functioncr.FunctionStateError, f.Status.State)
		suite.Equal("Validation failed: Cannot specify function version in spec on a created function (3)", f.Status.Message)

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
	suite.Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerCreateTestSuite) TestCreateStatusAndMessageSet() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Spec.Alias = "wrong"

	err := suite.controller.addFunction(&function)
	suite.Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerCreateTestSuite) TestLatestCreateInvalidVersionInSpec() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Spec.Version = 50

	err := suite.controller.addFunction(&function)
	suite.Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerCreateTestSuite) TestLatestCreateInvalidVersionInName() {
	function := functioncr.Function{}
	function.Name = "funcname-30"

	err := suite.controller.addFunction(&function)
	suite.Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerCreateTestSuite) TestLatestCreateInvalidAlias() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Spec.Alias = "wrong"

	err := suite.controller.addFunction(&function)
	suite.Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

//
// Update
//

type ControllerUpdateTestSuite struct {
	ControllerTestSuite
}

//func (suite *ControllerCreateTestSuite) TestLatestPublish() {
//	function := functioncr.Function{}
//	function.Name = "funcname"
//	function.Namespace = "funcnamespace"
//	function.ResourceVersion = "123"
//	function.Spec.Publish = true
//
//	// verify that fields were updated on function cr
//	verifyUpdatedFunctioncr := func(f *functioncr.Function) bool {
//		suite.Equal("funcname", f.GetLabels()["name"])
//		suite.Equal("latest", f.GetLabels()["version"])
//		suite.Equal(-1, f.Spec.Version)
//		suite.Equal("latest", f.Spec.Alias)
//		suite.Equal(functioncr.FunctionStateProcessed, f.Status.State)
//		suite.Equal("123", f.ResourceVersion)
//		suite.False(f.Spec.Publish)
//
//		return true
//	}
//
//	// expect update to happen on cr
//	suite.mockFunctioncrClient.
//		On("Update", mock.MatchedBy(verifyUpdatedFunctioncr)).
//		Return(&function, nil).
//		Once()
//
//	// expect resource version to be ignored
//	suite.mockChangeIgnorer.
//		On("Push", "funcnamespace.funcname", "123").
//		Once()
//
//	// expect a function deployment to be created
//	suite.mockFunctiondepClient.
//		On("CreateOrUpdate", mock.MatchedBy(verifyUpdatedFunctioncr)).
//		Return(&v1beta1.Deployment{}, nil).
//		Once()
//
//	verifyPublishedFunctioncr := func(f *functioncr.Function) bool {
//		suite.Equal("funcname-0", f.Name)
//		suite.False(f.Spec.Publish)
//		suite.Equal("", f.ResourceVersion)
//		suite.Equal("", f.Spec.Alias)
//		suite.Equal("funcname", function.GetLabels()["name"])
//		suite.Equal("0", f.GetLabels()["version"])
//		suite.Equal(functioncr.FunctionStateProcessed, f.Status.State)
//
//		return true
//	}
//
//	publishedFunction := function
//	publishedFunction.Name = "funcname-0"
//	publishedFunction.ResourceVersion = "555"
//
//	// expect a function cr to be created. return a publishedFunction so we can test it is ignored
//	suite.mockFunctioncrClient.
//		On("Create", mock.MatchedBy(verifyPublishedFunctioncr)).
//		Return(&publishedFunction, nil).
//		Once()
//
//	// expect created function
//	suite.mockChangeIgnorer.
//		On("Push", "funcnamespace.funcname-0", "555").
//		Once()
//
//	// expect a function deployment to be created
//	suite.mockFunctiondepClient.
//		On("CreateOrUpdate", mock.Anything).
//		Return(&v1beta1.Deployment{}, nil).
//		Once()
//
//	err := suite.controller.updateFunction(&function)
//	suite.NoError(err)
//
//	// make sure all expectations are met
//	suite.mockFunctioncrClient.AssertExpectations(suite.T())
//}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerCreateTestSuite))
}
