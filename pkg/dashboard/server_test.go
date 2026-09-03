//go:build test_unit

/*
Copyright 2023 The Nuclio Authors.

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
	"context"
	"fmt"
	"testing"

	"github.com/nuclio/nuclio/pkg/dockercreds"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform"
	mockplatform "github.com/nuclio/nuclio/pkg/platform/mock"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	opaclient "github.com/nuclio/opa-client"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type DashboardServerTestSuite struct {
	suite.Suite
	Server
	mockPlatform *mockplatform.Platform
	Logger       logger.Logger
}

func (suite *DashboardServerTestSuite) SetupTest() {
	var err error

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.mockPlatform = &mockplatform.Platform{}
	suite.Platform = suite.mockPlatform
	suite.defaultNamespace = "default-namespace"
	suite.platformConfiguration = &platformconfig.Config{
		Opa: &opa.Config{
			Config: &opaclient.Config{OverrideHeaderValue: "iguazio"},
		},
	}

	// the sweep logs via the embedded AbstractServer's logger, so it must be wired in the test
	suite.AbstractServer = &restful.AbstractServer{Logger: suite.Logger}
}

func (suite *DashboardServerTestSuite) TestResolveRegistryURLFromDockerCredentials() {
	dummyUsername := "dummy-user"
	for _, testCase := range []struct {
		credentials         dockercreds.Credentials
		expectedRegistryURL string
		Match               bool
	}{
		{
			credentials:         dockercreds.Credentials{URL: "https://index.docker.io/v1/", Username: dummyUsername},
			expectedRegistryURL: fmt.Sprintf("index.docker.io/%s", dummyUsername),
		},
		{
			credentials:         dockercreds.Credentials{URL: "index.docker.io/v1/", Username: dummyUsername},
			expectedRegistryURL: fmt.Sprintf("index.docker.io/%s", dummyUsername),
		},
		{
			credentials:         dockercreds.Credentials{URL: "https://index.docker.io", Username: dummyUsername},
			expectedRegistryURL: fmt.Sprintf("index.docker.io/%s", dummyUsername),
		},
		{
			credentials:         dockercreds.Credentials{URL: "index.docker.io", Username: dummyUsername},
			expectedRegistryURL: fmt.Sprintf("index.docker.io/%s", dummyUsername),
		},
		{
			credentials:         dockercreds.Credentials{URL: fmt.Sprintf("index.docker.io/%s", dummyUsername), Username: dummyUsername},
			expectedRegistryURL: fmt.Sprintf("index.docker.io/%s", dummyUsername),
		},
		{
			credentials:         dockercreds.Credentials{URL: "index.docker.io/another-username", Username: dummyUsername},
			expectedRegistryURL: "index.docker.io/another-username",
		},
	} {
		suite.Require().Equal(testCase.expectedRegistryURL, suite.resolveDockerCredentialsRegistryURL(testCase.credentials))
	}
}

// TestMarkStaleFunctionsAsError verifies that functions in pre-build/build states are flipped to
// error while functions in other states are left untouched.
func (suite *DashboardServerTestSuite) TestMarkStaleFunctionsAsError() {
	buildingFunction := suite.newFunction("f-building", functionconfig.FunctionStateBuilding)
	waitingForBuildFunction := suite.newFunction("f-waiting", functionconfig.FunctionStateWaitingForBuild)
	readyFunction := suite.newFunction("f-ready", functionconfig.FunctionStateReady)

	verifyGetFunctions := func(getFunctionsOptions *platform.GetFunctionsOptions) bool {
		return getFunctionsOptions.Namespace == "default-namespace"
	}

	// names of functions we expect to be flipped to error
	expectedFlipped := map[string]bool{
		"f-building": false,
		"f-waiting":  false,
	}
	verifyUpdateFunction := func(updateFunctionOptions *platform.UpdateFunctionOptions) bool {
		name := updateFunctionOptions.FunctionMeta.Name
		_, expected := expectedFlipped[name]
		suite.Require().True(expected, "unexpected function flipped to error: %s", name)
		suite.Require().Equal(functionconfig.FunctionStateError, updateFunctionOptions.FunctionStatus.State)
		suite.Require().NotEmpty(updateFunctionOptions.FunctionStatus.Message)

		// status only: the spec was read before the sweep started, so sending it back would revert any spec
		// change made in between
		suite.Require().Nil(updateFunctionOptions.FunctionSpec)

		expectedFlipped[name] = true
		return true
	}

	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.MatchedBy(verifyGetFunctions)).
		Return([]platform.Function{buildingFunction, waitingForBuildFunction, readyFunction}, nil).
		Once()
	suite.mockPlatform.
		On("UpdateFunction", mock.Anything, mock.MatchedBy(verifyUpdateFunction)).
		Return(nil).
		Twice()

	suite.markStaleFunctionsAsError(context.Background())

	suite.mockPlatform.AssertExpectations(suite.T())
	for name, flipped := range expectedFlipped {
		suite.Require().True(flipped, "expected function to be flipped to error: %s", name)
	}
}

// TestMarkStaleFunctionsAsErrorSuppressesReadinessError verifies that the expected
// "in error state" readiness-wait error returned by the kube platform's UpdateFunction (after the
// status was already persisted) is treated as success and does not abort the sweep.
func (suite *DashboardServerTestSuite) TestMarkStaleFunctionsAsErrorSuppressesReadinessError() {
	buildingFunction := suite.newFunction("f-building", functionconfig.FunctionStateBuilding)

	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.Anything).
		Return([]platform.Function{buildingFunction}, nil).
		Once()

	// mimic kube updater: status is persisted, then waitForFunctionReadiness returns this error
	readinessErr := errors.Wrap(
		errors.New("NuclioFunction in error state:\nFunction deployment was interrupted"),
		"Failed to wait for function readiness")
	suite.mockPlatform.
		On("UpdateFunction", mock.Anything, mock.Anything).
		Return(readinessErr).
		Once()

	suite.markStaleFunctionsAsError(context.Background())

	suite.mockPlatform.AssertExpectations(suite.T())
}

// TestMarkStaleFunctionsAsErrorGetFunctionsError verifies that a failure to list functions is
// non-fatal: the sweep returns without panicking and never attempts to update any function.
func (suite *DashboardServerTestSuite) TestMarkStaleFunctionsAsErrorGetFunctionsError() {
	suite.mockPlatform.
		On("GetFunctions", mock.Anything, mock.Anything).
		Return([]platform.Function{}, errors.New("failed to list functions")).
		Once()

	suite.Require().NotPanics(func() {
		suite.markStaleFunctionsAsError(context.Background())
	})

	suite.mockPlatform.AssertExpectations(suite.T())
	suite.mockPlatform.AssertNotCalled(suite.T(), "UpdateFunction", mock.Anything, mock.Anything)
}

func (suite *DashboardServerTestSuite) newFunction(name string,
	state functionconfig.FunctionState) *platform.AbstractFunction {
	function := &platform.AbstractFunction{}
	function.Config.Meta.Name = name
	function.Config.Meta.Namespace = "default-namespace"
	function.Status.State = state
	return function
}

func TestDashboardServerTestSuite(t *testing.T) {
	suite.Run(t, new(DashboardServerTestSuite))
}
