package app

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/nuclio/nuclio/pkg/functioncr"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1beta1 "k8s.io/api/apps/v1beta1"
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

func (mfcc *MockFunctioncrClient) WatchForChanges(changeChan chan functioncr.Change) (*functioncr.Watcher, error) {
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

func (mfcc *MockFunctioncrClient) List(namespace string) (*functioncr.FunctionList, error) {
	args := mfcc.Called(namespace)
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
	logger nuclio.Logger
	controller Controller
	mockFunctioncrClient MockFunctioncrClient
	mockFunctiondepClient MockFunctiondepClient
	mockChangeIgnorer MockChangeIgnorer
}

func (suite *ControllerTestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZap("test", nucliozap.DebugLevel)

	// manually create a controller
	suite.controller = Controller{
		logger: suite.logger,
		functioncrClient: &suite.mockFunctioncrClient,
		functiondepClient: &suite.mockFunctiondepClient,
		ignoredFunctionCRChanges: &suite.mockChangeIgnorer,
	}
}

func (suite *ControllerTestSuite) TestLatestCreateSuccessful() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Namespace = "funcnamespace"
	function.ResourceVersion = "123"

	// verify that fields were updated on function cr
	verifyUpdatedFunctioncr := func(f *functioncr.Function) bool {
		suite.Equal("funcname", function.GetLabels()["name"])
		suite.Equal("latest", function.GetLabels()["version"])
		suite.Equal(0, function.Spec.Version)
		suite.Equal("latest", function.Spec.Alias)
		suite.Equal(functioncr.FunctionStateProcessed, f.Status.State)

		return  true
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

	err := suite.controller.addFunctioncr(&function)
	suite.NoError(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerTestSuite) TestLatestCreateAndPublish() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Namespace = "funcnamespace"
	function.ResourceVersion = "123"
	function.Spec.Publish = true

	// verify that fields were updated on function cr
	verifyUpdatedFunctioncr := func(f *functioncr.Function) bool {
		suite.Equal("funcname", function.GetLabels()["name"])
		suite.Equal("latest", function.GetLabels()["version"])
		suite.Equal(0, function.Spec.Version)
		suite.Equal("latest", function.Spec.Alias)
		suite.Equal(functioncr.FunctionStateProcessed, f.Status.State)
		suite.False(function.Spec.Publish)

		return  true
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

	verifyPublishedFunctioncr := func(f *functioncr.Function) bool {
		suite.Equal("funcname-0", f.Name)
		suite.False(function.Spec.Publish)

		return  true
	}

	// expect a function cr to be created
	suite.mockFunctioncrClient.
		On("Create", mock.MatchedBy(verifyPublishedFunctioncr)).
		Return(&function, nil).
		Once()

	err := suite.controller.addFunctioncr(&function)
	suite.NoError(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerTestSuite) TestCreateStatusAndMessageSet() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Spec.Alias = "wrong"

	err := suite.controller.addFunctioncr(&function)
	suite.Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerTestSuite) TestLatestCreateInvalidVersionInSpec() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Spec.Version = 50

	err := suite.controller.addFunctioncr(&function)
	suite.Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerTestSuite) TestLatestCreateInvalidVersionInName() {
	function := functioncr.Function{}
	function.Name = "funcname-30"

	err := suite.controller.addFunctioncr(&function)
	suite.Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

func (suite *ControllerTestSuite) TestLatestCreateInvalidAlias() {
	function := functioncr.Function{}
	function.Name = "funcname"
	function.Spec.Alias = "wrong"

	err := suite.controller.addFunctioncr(&function)
	suite.Error(err)

	// make sure all expectations are met
	suite.mockFunctioncrClient.AssertExpectations(suite.T())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}
