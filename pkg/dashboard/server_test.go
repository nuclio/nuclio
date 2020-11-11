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

package dashboard

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/dockercreds"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	mockplatform "github.com/nuclio/nuclio/pkg/platform/mock"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type DashboardServerTestSuite struct {
	suite.Suite
	Server
	mockPlatform *mockplatform.Platform
	Logger logger.Logger
}

func (suite *DashboardServerTestSuite) SetupTest() {
	var err error

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.mockPlatform = &mockplatform.Platform{}
	suite.Platform = suite.mockPlatform
}

func (suite *DashboardServerTestSuite) TestResolveRegistryURLFromDockerCredentials() {
	dummyUsername := "dummy-user"
	for _, testCase := range []struct {
		credentials             dockercreds.Credentials
		expectedRegistryURLHost string
		Match                   bool
	}{
		{
			credentials:             dockercreds.Credentials{URL: "https://index.docker.io/v1/", Username: dummyUsername},
			expectedRegistryURLHost: "index.docker.io",
		},
		{
			credentials:             dockercreds.Credentials{URL: "index.docker.io/v1/", Username: dummyUsername},
			expectedRegistryURLHost: "index.docker.io",
		},
		{
			credentials:             dockercreds.Credentials{URL: "https://index.docker.io", Username: dummyUsername},
			expectedRegistryURLHost: "index.docker.io",
		},
		{
			credentials:             dockercreds.Credentials{URL: "index.docker.io", Username: dummyUsername},
			expectedRegistryURLHost: "index.docker.io",
		},
	} {
		expectedRegistryURL := testCase.expectedRegistryURLHost + "/" + dummyUsername
		suite.Require().Equal(expectedRegistryURL, suite.resolveDockerCredentialsRegistryURL(testCase.credentials))
	}
}

func (suite *DashboardServerTestSuite) TestUpdateStuckFunctionsState() {
	returnedFunctionBuilding := platform.AbstractFunction{}
	returnedFunctionBuilding.Config.Meta.Name = "f1"
	returnedFunctionBuilding.Config.Meta.Namespace = "default-namespace"
	returnedFunctionBuilding.Status.State = functionconfig.FunctionStateBuilding
	returnedFunctionReady := platform.AbstractFunction{}
	returnedFunctionReady.Config.Meta.Name = "f2"
	returnedFunctionReady.Config.Meta.Namespace = "default-namespace"
	returnedFunctionReady.Status.State = functionconfig.FunctionStateReady

	suite.defaultNamespace = "default-namespace"

	// verify
	verifyGetFunctions := func(getFunctionsOptions *platform.GetFunctionsOptions) bool {
		suite.Require().Equal("default-namespace", getFunctionsOptions.Namespace)

		return true
	}
	verifyUpdateFunction := func(updateFunctionsOptions *platform.UpdateFunctionOptions) bool {
		suite.Require().Equal(returnedFunctionBuilding.GetConfig().Meta.Name, updateFunctionsOptions.FunctionMeta.Name)
		suite.Require().Equal(functionconfig.FunctionStateError, updateFunctionsOptions.FunctionStatus.State)

		return true
	}

	// mock returned functions
	suite.mockPlatform.
		On("GetFunctions", mock.MatchedBy(verifyGetFunctions)).
		Return([]platform.Function{&returnedFunctionBuilding, &returnedFunctionReady}, nil).
		Once()

	// mock update function - expect it to be called only once, for the function with "building" state
	suite.mockPlatform.
		On("UpdateFunction", mock.MatchedBy(verifyUpdateFunction)).
		Return(nil).
		Once()

	err := suite.updateStuckFunctionsState()
	suite.Assert().NoError(err)
}

func TestDashboardServerTestSuite(t *testing.T) {
	suite.Run(t, new(DashboardServerTestSuite))
}
