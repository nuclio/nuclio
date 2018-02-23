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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/dashboard"
	_ "github.com/nuclio/nuclio/pkg/dashboard/resource"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"
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

type DashboardTestSuite struct {
	suite.Suite
	logger logger.Logger
	dashboardServer *dashboard.Server
	httpServer *httptest.Server
	mockPlatform mockPlatform
}

func (suite *DashboardTestSuite) SetupTest() {
	var err error

	suite.logger, _ = nucliozap.NewNuclioZapTest("test")

	// create a mock platform
	suite.dashboardServer, err = dashboard.NewServer(suite.logger,
		"",
		"",
		"",
		"",
		&suite.mockPlatform,
		true,
		&platformconfig.WebServer{},
		nil)

	if err != nil {
		panic("Failed to create server")
	}

	// create an http server from the dashboard server
	suite.httpServer = httptest.NewServer(suite.dashboardServer.Router)
}

func (suite *DashboardTestSuite) TeardownTest() {
	suite.httpServer.Close()
}

func (suite *DashboardTestSuite) TestCreateSuccessful() {
	//suite.mockPlatform.
	//	On("GetFunctions", mock.MatchedBy(verifyFunctioncr)).
	//	Return(&v1beta1.Deployment{}, nil).
	//	Once()

	http.Get(suite.httpServer.URL + "/functions")
}

func TestInlineParserTestSuite(t *testing.T) {
	suite.Run(t, new(DashboardTestSuite))
}
