//go:build test_unit

/*
Copyright 2026 The Nuclio Authors.

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

package kube

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/opa-client"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AuthMigrationKubePlatformTestSuite struct {
	KubePlatformTestSuite

	previousAuthenticationConfig *platformconfig.Authentication
}

func (suite *AuthMigrationKubePlatformTestSuite) SetupSuite() {
	suite.KubePlatformTestSuite.SetupSuite()
}

func (suite *AuthMigrationKubePlatformTestSuite) SetupTest() {
	suite.KubePlatformTestSuite.SetupTest()

	// the sweep resolves the namespace itself, so it must match the one the CRD mocks are wired for
	suite.abstractPlatform.DefaultNamespace = suite.Namespace

	// every test declares the authentication configuration it needs, so start from an empty one - which also
	// leaves the feature flag off by default
	suite.previousAuthenticationConfig = suite.abstractPlatform.Config.Authentication
	suite.abstractPlatform.Config.Authentication = &platformconfig.Authentication{}
}

func (suite *AuthMigrationKubePlatformTestSuite) TearDownTest() {
	suite.abstractPlatform.Config.Authentication = suite.previousAuthenticationConfig
}

// TestResolveModeFromSingleAPIGateway covers the legacy-to-function-level mode translation.
func (suite *AuthMigrationKubePlatformTestSuite) TestResolveModeFromSingleAPIGateway() {
	for _, testCase := range []struct {
		name         string
		apiGateway   *v1beta1.NuclioAPIGateway
		defaultMode  auth.AuthenticationMode
		expectedMode auth.AuthenticationMode
	}{
		{
			name:         "iguazio redirects, so browser",
			apiGateway:   suite.compileAPIGateway("gw", "func", auth.AuthenticationModeIguazio, nil),
			defaultMode:  auth.AuthenticationModeAPI,
			expectedMode: auth.AuthenticationModeBrowser,
		},
		{
			name:         "accessKey does not redirect, so api",
			apiGateway:   suite.compileAPIGateway("gw", "func", auth.AuthenticationModeAccessKey, nil),
			defaultMode:  auth.AuthenticationModeAPI,
			expectedMode: auth.AuthenticationModeAPI,
		},
		{
			name: "oauth2 with redirect is browser",
			apiGateway: suite.compileAPIGateway("gw", "func", auth.AuthenticationModeOauth2,
				suite.compileDexAuthentication(true)),
			defaultMode:  auth.AuthenticationModeAPI,
			expectedMode: auth.AuthenticationModeBrowser,
		},
		{
			name: "oauth2 without redirect is api",
			apiGateway: suite.compileAPIGateway("gw", "func", auth.AuthenticationModeOauth2,
				suite.compileDexAuthentication(false)),
			defaultMode:  auth.AuthenticationModeAPI,
			expectedMode: auth.AuthenticationModeAPI,
		},
		{
			name: "basicAuth stays basicAuth",
			apiGateway: suite.compileAPIGateway("gw", "func", auth.AuthenticationModeBasicAuth,
				suite.compileBasicAuthentication("user", "pass")),
			defaultMode:  auth.AuthenticationModeAPI,
			expectedMode: auth.AuthenticationModeBasicAuth,
		},
		{
			name:         "a gateway with no authentication falls back to the platform default",
			apiGateway:   suite.compileAPIGateway("gw", "func", auth.AuthenticationModeNone, nil),
			defaultMode:  auth.AuthenticationModeAPI,
			expectedMode: auth.AuthenticationModeAPI,
		},
		{
			name:         "an unknown gateway mode fails secure to the platform default",
			apiGateway:   suite.compileAPIGateway("gw", "func", auth.AuthenticationMode("bogus"), nil),
			defaultMode:  auth.AuthenticationModeAPI,
			expectedMode: auth.AuthenticationModeAPI,
		},
		{
			name:         "a none default is written explicitly, never left empty",
			apiGateway:   suite.compileAPIGateway("gw", "func", auth.AuthenticationModeNone, nil),
			defaultMode:  auth.AuthenticationModeNone,
			expectedMode: auth.AuthenticationModeNone,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.abstractPlatform.Config.Authentication.DefaultMode = testCase.defaultMode

			migrations := suite.platform.resolveFunctionAuthMigrations(suite.ctx,
				[]v1beta1.NuclioFunction{
					*suite.compileFunction("func", functionconfig.FunctionStateReady, ""),
				},
				[]v1beta1.NuclioAPIGateway{*testCase.apiGateway})

			suite.Require().Len(migrations, 1)
			suite.Require().Equal(testCase.expectedMode, migrations["func"].mode)
		})
	}
}

// TestResolveModeFromDivergentAPIGateways covers the priority rule: the platform default wins, then the
// other credential-less mode, then basicAuth.
func (suite *AuthMigrationKubePlatformTestSuite) TestResolveModeFromDivergentAPIGateways() {
	for _, testCase := range []struct {
		name         string
		gatewayModes []auth.AuthenticationMode
		defaultMode  auth.AuthenticationMode
		expectedMode auth.AuthenticationMode
	}{
		{
			name:         "api default beats browser",
			gatewayModes: []auth.AuthenticationMode{auth.AuthenticationModeIguazio, auth.AuthenticationModeAccessKey},
			defaultMode:  auth.AuthenticationModeAPI,
			expectedMode: auth.AuthenticationModeAPI,
		},
		{
			name:         "browser default beats api",
			gatewayModes: []auth.AuthenticationMode{auth.AuthenticationModeAccessKey, auth.AuthenticationModeIguazio},
			defaultMode:  auth.AuthenticationModeBrowser,
			expectedMode: auth.AuthenticationModeBrowser,
		},
		{
			name:         "basicAuth is always last",
			gatewayModes: []auth.AuthenticationMode{auth.AuthenticationModeBasicAuth, auth.AuthenticationModeIguazio},
			defaultMode:  auth.AuthenticationModeAPI,
			expectedMode: auth.AuthenticationModeBrowser,
		},
		{
			name:         "basicAuth wins when it is the only mode configured",
			gatewayModes: []auth.AuthenticationMode{auth.AuthenticationModeNone, auth.AuthenticationModeBasicAuth},
			defaultMode:  auth.AuthenticationModeAPI,
			expectedMode: auth.AuthenticationModeBasicAuth,
		},
		{
			name:         "authentication always beats none",
			gatewayModes: []auth.AuthenticationMode{auth.AuthenticationModeNone, auth.AuthenticationModeIguazio},
			defaultMode:  auth.AuthenticationModeNone,
			expectedMode: auth.AuthenticationModeBrowser,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.abstractPlatform.Config.Authentication.DefaultMode = testCase.defaultMode

			var apiGateways []v1beta1.NuclioAPIGateway
			for gatewayIndex, gatewayMode := range testCase.gatewayModes {
				var authentication *platform.APIGatewayAuthenticationSpec
				if gatewayMode == auth.AuthenticationModeBasicAuth {
					authentication = suite.compileBasicAuthentication("user", "pass")
				}
				apiGateways = append(apiGateways, *suite.compileAPIGateway(fmt.Sprintf("gw-%d", gatewayIndex),
					"func",
					gatewayMode,
					authentication))
			}

			migrations := suite.platform.resolveFunctionAuthMigrations(suite.ctx,
				[]v1beta1.NuclioFunction{
					*suite.compileFunction("func", functionconfig.FunctionStateReady, ""),
				},
				apiGateways)

			suite.Require().Len(migrations, 1)
			suite.Require().Equal(testCase.expectedMode, migrations["func"].mode)

			// the credential source is only ever read once the winning mode is basicAuth, so it is
			// deliberately left claimed by an outranked gateway rather than cleared
			if testCase.expectedMode == auth.AuthenticationModeBasicAuth {
				suite.Require().NotNil(migrations["func"].basicAuthAPIGateway)
			}
		})
	}
}

// TestResolveModeForCanaryAPIGateway asserts a canary gateway contributes its mode to both of its targets.
func (suite *AuthMigrationKubePlatformTestSuite) TestResolveModeForCanaryAPIGateway() {
	suite.abstractPlatform.Config.Authentication.DefaultMode = auth.AuthenticationModeAPI

	canaryAPIGateway := suite.compileAPIGateway("gw", "primary-func", auth.AuthenticationModeIguazio, nil)
	canaryAPIGateway.Spec.Upstreams = append(canaryAPIGateway.Spec.Upstreams, platform.APIGatewayUpstreamSpec{
		Kind:           platform.APIGatewayUpstreamKindNuclioFunction,
		NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{Name: "canary-func"},
		Percentage:     20,
	})

	for _, functionName := range []string{"primary-func", "canary-func"} {
		migrations := suite.platform.resolveFunctionAuthMigrations(suite.ctx,
			[]v1beta1.NuclioFunction{
				*suite.compileFunction(functionName, functionconfig.FunctionStateReady, ""),
			},
			[]v1beta1.NuclioAPIGateway{*canaryAPIGateway})

		suite.Require().Len(migrations, 1)
		suite.Require().Equal(auth.AuthenticationModeBrowser, migrations[functionName].mode, functionName)
	}
}

// TestResolveModeSkipsAlreadyMigratedFunctions asserts a function that is already on the new model, or has
// no HTTP trigger to carry a mode, is migrated by labeling it alone.
func (suite *AuthMigrationKubePlatformTestSuite) TestResolveModeSkipsAlreadyMigratedFunctions() {
	suite.abstractPlatform.Config.Authentication.DefaultMode = auth.AuthenticationModeAPI
	apiGateway := *suite.compileAPIGateway("gw", "func", auth.AuthenticationModeIguazio, nil)
	for _, testCase := range []struct {
		name     string
		function *v1beta1.NuclioFunction
	}{
		{
			name:     "an explicit mode is not overwritten",
			function: suite.compileFunction("func", functionconfig.FunctionStateReady, auth.AuthenticationModeNone),
		},
		{
			name: "a function with no HTTP trigger has nowhere to put a mode",
			function: func() *v1beta1.NuclioFunction {
				function := suite.compileFunction("func", functionconfig.FunctionStateReady, "")
				function.Spec.Triggers = map[string]functionconfig.Trigger{}
				return function
			}(),
		},
	} {
		suite.Run(testCase.name, func() {
			migrations := suite.platform.resolveFunctionAuthMigrations(suite.ctx,
				[]v1beta1.NuclioFunction{*testCase.function},
				[]v1beta1.NuclioAPIGateway{apiGateway})

			suite.Require().Len(migrations, 1)
			suite.Require().Equal(auth.AuthenticationMode(""), migrations["func"].mode)
		})
	}
}

// TestMigrateLabelsStaleFunctionsWithoutMode asserts a mid-deploy function is labeled but keeps its spec
// untouched, so the redeploy the user has to run is not blocked by the migration gate and sets the mode
// itself.
func (suite *AuthMigrationKubePlatformTestSuite) TestMigrateLabelsStaleFunctionsWithoutMode() {
	suite.withFunctionAuthentication(auth.AuthenticationModeAPI)

	for _, testCase := range []struct {
		functionState functionconfig.FunctionState
		expectModeSet bool
	}{
		{functionState: functionconfig.FunctionStateReady, expectModeSet: true},
		{functionState: functionconfig.FunctionStateScaledToZero, expectModeSet: true},
		{functionState: functionconfig.FunctionStateError, expectModeSet: true},
		{functionState: functionconfig.FunctionStateImported, expectModeSet: true},
		{functionState: functionconfig.FunctionStateUnhealthy, expectModeSet: true},
		{functionState: functionconfig.FunctionStateWaitingForResourceConfiguration, expectModeSet: true},
		{functionState: functionconfig.FunctionStateConfiguringResources, expectModeSet: true},
		{functionState: functionconfig.FunctionStateWaitingForScaleResourcesToZero, expectModeSet: true},
		// mid-deploy, so the mode is left to the redeploy that follows
		{functionState: functionconfig.FunctionStateBuilding, expectModeSet: false},
		{functionState: functionconfig.FunctionStateWaitingForBuild, expectModeSet: false},
	} {
		suite.Run(string(testCase.functionState), func() {
			suite.ResetCRDMocks()
			suite.abstractPlatform.DefaultNamespace = suite.Namespace

			function := suite.compileFunction("func", testCase.functionState, "")
			suite.expectListUnmigrated([]v1beta1.NuclioFunction{*function}, nil)

			updatedFunction := &v1beta1.NuclioFunction{}
			suite.nuclioFunctionInterfaceMock.
				On("Update", suite.ctx, mock.MatchedBy(func(function *v1beta1.NuclioFunction) bool {
					*updatedFunction = *function
					return true
				}), metav1.UpdateOptions{}).
				Return(updatedFunction, nil).
				Once()

			suite.platform.MigrateFunctionAuthentication(suite.ctx)

			if testCase.expectModeSet {
				suite.Require().Equal(string(auth.AuthenticationModeAPI),
					updatedFunction.Spec.Triggers["http"].Attributes[auth.AttributeAuthenticationMode])
			} else {
				suite.Require().Empty(
					updatedFunction.Spec.Triggers["http"].Attributes[auth.AttributeAuthenticationMode])
			}

			// every function is labeled either way, so none of them stays behind the write gate
			suite.Require().Equal(common.NuclioLabelValueMigrationApplied,
				updatedFunction.Labels[common.NuclioLabelKeyMigrationFunctionAuth])
			suite.nuclioFunctionInterfaceMock.AssertExpectations(suite.T())
		})
	}
}

// TestMigrateFunctionStampsModeAndLabel is the end-to-end happy path over the CRD client.
func (suite *AuthMigrationKubePlatformTestSuite) TestMigrateFunctionStampsModeAndLabel() {
	suite.withFunctionAuthentication(auth.AuthenticationModeAPI)

	function := suite.compileFunction("func", functionconfig.FunctionStateReady, "")
	apiGateway := suite.compileAPIGateway("gw", "func", auth.AuthenticationModeIguazio, nil)
	suite.expectListUnmigrated([]v1beta1.NuclioFunction{*function}, []v1beta1.NuclioAPIGateway{*apiGateway})

	updatedFunction := &v1beta1.NuclioFunction{}
	suite.nuclioFunctionInterfaceMock.
		On("Update", suite.ctx, mock.MatchedBy(func(function *v1beta1.NuclioFunction) bool {
			*updatedFunction = *function
			return true
		}), metav1.UpdateOptions{}).
		Return(updatedFunction, nil).
		Once()
	updatedAPIGateway := suite.expectAPIGatewayUpdate()

	suite.platform.MigrateFunctionAuthentication(suite.ctx)

	suite.Require().Equal(string(auth.AuthenticationModeBrowser),
		updatedFunction.Spec.Triggers["http"].Attributes[auth.AttributeAuthenticationMode])
	suite.Require().Equal(common.NuclioLabelValueMigrationApplied,
		updatedFunction.Labels[common.NuclioLabelKeyMigrationFunctionAuth])

	// the gateway keeps its upstreams and loses only its authentication
	suite.Require().Equal(auth.AuthenticationMode(""), updatedAPIGateway.Spec.AuthenticationMode)
	suite.Require().Nil(updatedAPIGateway.Spec.Authentication)
	suite.Require().Len(updatedAPIGateway.Spec.Upstreams, 1)

	// and is re-provisioned, so the controller re-renders its ingress without the nginx auth annotations
	suite.Require().Equal(platform.APIGatewayStateWaitingForProvisioning, updatedAPIGateway.Status.State)
	suite.Require().Equal(common.NuclioLabelValueMigrationApplied,
		updatedAPIGateway.Labels[common.NuclioLabelKeyMigrationFunctionAuth])

	suite.nuclioFunctionInterfaceMock.AssertExpectations(suite.T())
	suite.nuclioAPIGatewayInterfaceMock.AssertExpectations(suite.T())
}

// TestMigrateFunctionWithNoAPIGatewayUsesPlatformDefault asserts a function no gateway speaks for still
// ends up with a mode - the platform-wide default - rather than being left with none.
func (suite *AuthMigrationKubePlatformTestSuite) TestMigrateFunctionWithNoAPIGatewayUsesPlatformDefault() {
	for _, defaultMode := range []auth.AuthenticationMode{
		auth.AuthenticationModeAPI,
		auth.AuthenticationModeBrowser,
		auth.AuthenticationModeNone,
	} {
		suite.Run(string(defaultMode), func() {
			suite.ResetCRDMocks()
			suite.abstractPlatform.DefaultNamespace = suite.Namespace
			suite.withFunctionAuthentication(defaultMode)

			function := suite.compileFunction("func", functionconfig.FunctionStateReady, "")
			suite.expectListUnmigrated([]v1beta1.NuclioFunction{*function}, nil)

			updatedFunction := &v1beta1.NuclioFunction{}
			suite.nuclioFunctionInterfaceMock.
				On("Update", suite.ctx, mock.MatchedBy(func(function *v1beta1.NuclioFunction) bool {
					*updatedFunction = *function
					return true
				}), metav1.UpdateOptions{}).
				Return(updatedFunction, nil).
				Once()

			suite.platform.MigrateFunctionAuthentication(suite.ctx)

			suite.Require().Equal(string(defaultMode),
				updatedFunction.Spec.Triggers["http"].Attributes[auth.AttributeAuthenticationMode])
			suite.Require().Equal(common.NuclioLabelValueMigrationApplied,
				updatedFunction.Labels[common.NuclioLabelKeyMigrationFunctionAuth])
			suite.nuclioFunctionInterfaceMock.AssertExpectations(suite.T())
		})
	}
}

// TestMigrateFunctionAlreadyOnTheNewModelOnlyMarksIt asserts a function that already declares a mode is
// marked without its spec being touched, so the sweep never overwrites an explicit choice.
func (suite *AuthMigrationKubePlatformTestSuite) TestMigrateFunctionAlreadyOnTheNewModelOnlyMarksIt() {
	suite.withFunctionAuthentication(auth.AuthenticationModeAPI)
	function := suite.compileFunction("func", functionconfig.FunctionStateReady, auth.AuthenticationModeNone)
	apiGateway := suite.compileAPIGateway("gw", "func", auth.AuthenticationModeIguazio, nil)
	suite.expectListUnmigrated([]v1beta1.NuclioFunction{*function}, []v1beta1.NuclioAPIGateway{*apiGateway})

	updatedFunction := &v1beta1.NuclioFunction{}
	suite.nuclioFunctionInterfaceMock.
		On("Update", suite.ctx, mock.MatchedBy(func(function *v1beta1.NuclioFunction) bool {
			*updatedFunction = *function
			return true
		}), metav1.UpdateOptions{}).
		Return(updatedFunction, nil).
		Once()
	suite.expectAPIGatewayUpdate()

	suite.platform.MigrateFunctionAuthentication(suite.ctx)

	suite.Require().Equal(string(auth.AuthenticationModeNone),
		updatedFunction.Spec.Triggers["http"].Attributes[auth.AttributeAuthenticationMode])
	suite.Require().Equal(common.NuclioLabelValueMigrationApplied,
		updatedFunction.Labels[common.NuclioLabelKeyMigrationFunctionAuth])
	suite.nuclioFunctionInterfaceMock.AssertExpectations(suite.T())
}

// TestChooseAuthByPriority covers the rank-based tie-break directly.
func (suite *AuthMigrationKubePlatformTestSuite) TestChooseAuthByPriority() {
	for _, testCase := range []struct {
		defaultMode  auth.AuthenticationMode
		modeA        auth.AuthenticationMode
		modeB        auth.AuthenticationMode
		expectedMode auth.AuthenticationMode
	}{
		// the platform default outranks everything
		{auth.AuthenticationModeAPI, auth.AuthenticationModeBrowser, auth.AuthenticationModeAPI, auth.AuthenticationModeAPI},
		{auth.AuthenticationModeBrowser, auth.AuthenticationModeAPI, auth.AuthenticationModeBrowser, auth.AuthenticationModeBrowser},
		// basicAuth is last, since it is the only mode carrying per-function credentials
		{auth.AuthenticationModeAPI, auth.AuthenticationModeBasicAuth, auth.AuthenticationModeBrowser, auth.AuthenticationModeBrowser},
		{auth.AuthenticationModeAPI, auth.AuthenticationModeBasicAuth, "", auth.AuthenticationModeBasicAuth},
		// authentication always beats none
		{auth.AuthenticationModeNone, auth.AuthenticationModeNone, auth.AuthenticationModeBasicAuth, auth.AuthenticationModeBasicAuth},
		{auth.AuthenticationModeAPI, auth.AuthenticationModeAPI, auth.AuthenticationModeNone, auth.AuthenticationModeAPI},
		// with a none default there is still a deterministic order
		{auth.AuthenticationModeNone, auth.AuthenticationModeBrowser, auth.AuthenticationModeAPI, auth.AuthenticationModeAPI},
	} {
		suite.Run(fmt.Sprintf("default=%s/%s-vs-%s", testCase.defaultMode, testCase.modeA, testCase.modeB), func() {
			suite.abstractPlatform.Config.Authentication.DefaultMode = testCase.defaultMode
			suite.Require().Equal(testCase.expectedMode,
				suite.platform.chooseAuthByPriority(testCase.modeA, testCase.modeB))
		})
	}
}

// TestMigrateBasicAuthScrubsCredentialsOntoTheFunction asserts the password is copied by value into the
// function's own secret, since a $ref cannot be moved - it resolves only against the secret of its owner.
func (suite *AuthMigrationKubePlatformTestSuite) TestMigrateBasicAuthScrubsCredentialsOntoTheFunction() {
	suite.withFunctionAuthentication(auth.AuthenticationModeAPI)
	suite.abstractPlatform.Config.EnableSensitiveFieldMasking()
	// the test is not about the masking itself, so turn it off after the test to avoid polluting other tests
	suite.T().Cleanup(suite.abstractPlatform.Config.DisableSensitiveFieldMasking)

	function := suite.compileFunction("func", functionconfig.FunctionStateReady, "")
	apiGateway := suite.compileAPIGateway("gw", "func", auth.AuthenticationModeBasicAuth,
		suite.compileBasicAuthentication("some-user", "test-pass"))
	suite.expectListUnmigrated([]v1beta1.NuclioFunction{*function}, []v1beta1.NuclioAPIGateway{*apiGateway})

	updatedFunction := &v1beta1.NuclioFunction{}
	suite.nuclioFunctionInterfaceMock.
		On("Update", suite.ctx, mock.MatchedBy(func(function *v1beta1.NuclioFunction) bool {
			*updatedFunction = *function
			return true
		}), metav1.UpdateOptions{}).
		Return(updatedFunction, nil).
		Once()
	suite.expectAPIGatewayUpdate()

	suite.platform.MigrateFunctionAuthentication(suite.ctx)

	suite.Require().Equal(string(auth.AuthenticationModeBasicAuth),
		updatedFunction.Spec.Triggers["http"].Attributes[auth.AttributeAuthenticationMode])

	basicAuthAttributes := updatedFunction.Spec.Triggers["http"].
		Attributes[auth.AttributeAuthentication].(map[string]interface{})[auth.AttributeBasicAuth].(map[string]interface{})
	suite.Require().Equal("some-user", basicAuthAttributes["username"])

	// the password is stored as a reference into the function's own secret, never in plaintext on the CRD,
	// and the reference is the function-side field path rather than the gateway's
	passwordReference, ok := basicAuthAttributes["password"].(string)
	suite.Require().True(ok)
	suite.Require().Equal(
		functionconfig.ReferencePrefix+"/spec/triggers/http/attributes/authentication/basicauth/password",
		passwordReference)

	// asserted on the created secret rather than through GetObjectSecretMap, because the fake clientset does
	// not do the api server's StringData -> Data conversion
	functionScrubber := suite.platform.GetFunctionScrubber()
	functionSecrets, err := functionScrubber.GetObjectSecrets(suite.ctx, "func", suite.Namespace)
	suite.Require().NoError(err)
	suite.Require().Len(functionSecrets, 1)
	suite.Require().Equal(v1.SecretType(functionconfig.SecretTypeFunctionConfig), functionSecrets[0].Type)

	suite.Require().Equal("test-pass",
		functionSecrets[0].StringData[functionScrubber.EncodeSecretKey(passwordReference)])
}

// TestMigrateKeepsAPIGatewayAuthenticationWhenItsFunctionFails asserts a failed function write holds back
// only the gateways in front of that function - draining them would leave it unauthenticated, and the next
// restart could no longer derive its mode from them - while gateways whose functions all migrated are drained.
func (suite *AuthMigrationKubePlatformTestSuite) TestMigrateKeepsAPIGatewayAuthenticationWhenItsFunctionFails() {
	suite.withFunctionAuthentication(auth.AuthenticationModeAPI)

	migratedFunction := suite.compileFunction("migrated-func", functionconfig.FunctionStateReady, "")
	failedFunction := suite.compileFunction("failed-func", functionconfig.FunctionStateReady, "")

	// the canary gateway fronts both functions, so the one that fails holds it back on its own
	canaryAPIGateway := suite.compileAPIGateway("canary-gw", "migrated-func", auth.AuthenticationModeIguazio, nil)
	canaryAPIGateway.Spec.Upstreams = append(canaryAPIGateway.Spec.Upstreams, platform.APIGatewayUpstreamSpec{
		Kind:           platform.APIGatewayUpstreamKindNuclioFunction,
		NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{Name: "failed-func"},
	})
	migratedAPIGateway := suite.compileAPIGateway("migrated-gw", "migrated-func", auth.AuthenticationModeIguazio, nil)

	suite.expectListUnmigrated(
		[]v1beta1.NuclioFunction{*migratedFunction, *failedFunction},
		[]v1beta1.NuclioAPIGateway{*canaryAPIGateway, *migratedAPIGateway})

	suite.nuclioFunctionInterfaceMock.
		On("Update", suite.ctx, mock.MatchedBy(func(function *v1beta1.NuclioFunction) bool {
			return function.Name == "migrated-func"
		}), metav1.UpdateOptions{}).
		Return(migratedFunction, nil).
		Once()
	suite.nuclioFunctionInterfaceMock.
		On("Update", suite.ctx, mock.MatchedBy(func(function *v1beta1.NuclioFunction) bool {
			return function.Name == "failed-func"
		}), metav1.UpdateOptions{}).
		Return(nil, apierrors.NewBadRequest("failed to write the function")).
		Once()

	// deliberately matches any gateway, so the assertions below can pin down exactly which ones were drained
	var updatedAPIGateways []*v1beta1.NuclioAPIGateway
	var updatedAPIGatewaysLock sync.Mutex
	suite.nuclioAPIGatewayInterfaceMock.
		On("Update", suite.ctx, mock.Anything, metav1.UpdateOptions{}).
		Return(func(_ context.Context,
			apiGateway *v1beta1.NuclioAPIGateway,
			_ metav1.UpdateOptions) *v1beta1.NuclioAPIGateway {
			updatedAPIGatewaysLock.Lock()
			defer updatedAPIGatewaysLock.Unlock()
			updatedAPIGateways = append(updatedAPIGateways, apiGateway)
			return apiGateway
		}, nil)

	suite.platform.MigrateFunctionAuthentication(suite.ctx)

	// the canary gateway is left untouched - authentication, label and state - so the next restart lists it
	// again and re-derives the mode from it
	suite.Require().Len(updatedAPIGateways, 1)
	suite.Require().Equal("migrated-gw", updatedAPIGateways[0].Name)
	suite.Require().Equal(auth.AuthenticationMode(""), updatedAPIGateways[0].Spec.AuthenticationMode)
	suite.Require().Equal(common.NuclioLabelValueMigrationApplied,
		updatedAPIGateways[0].Labels[common.NuclioLabelKeyMigrationFunctionAuth])

	suite.nuclioFunctionInterfaceMock.AssertExpectations(suite.T())
	suite.nuclioAPIGatewayInterfaceMock.AssertExpectations(suite.T())
}

// TestMigrateIsIdempotent asserts a second sweep writes nothing, because the label selector excludes
// everything the first sweep stamped.
func (suite *AuthMigrationKubePlatformTestSuite) TestMigrateIsIdempotent() {
	suite.withFunctionAuthentication(auth.AuthenticationModeAPI)

	// the api server filters on the selector, so a second sweep sees empty lists
	suite.expectListUnmigrated(nil, nil)

	suite.platform.MigrateFunctionAuthentication(suite.ctx)

	suite.nuclioFunctionInterfaceMock.AssertNotCalled(suite.T(), "Update", mock.Anything, mock.Anything, mock.Anything)
	suite.nuclioAPIGatewayInterfaceMock.AssertNotCalled(suite.T(), "Update", mock.Anything, mock.Anything, mock.Anything)
}

// TestMigrateDisabledByFeatureFlag asserts the sweep is a provable no-op while the flag is off.
func (suite *AuthMigrationKubePlatformTestSuite) TestMigrateDisabledByFeatureFlag() {
	suite.platform.MigrateFunctionAuthentication(suite.ctx)

	suite.nuclioFunctionInterfaceMock.AssertNotCalled(suite.T(), "List", mock.Anything, mock.Anything)
	suite.nuclioAPIGatewayInterfaceMock.AssertNotCalled(suite.T(), "List", mock.Anything, mock.Anything)
}

// TestCheckNotMigratedToFunctionAuthentication covers the write gate in isolation.
func (suite *AuthMigrationKubePlatformTestSuite) TestCheckNotMigratedToFunctionAuthentication() {
	for _, testCase := range []struct {
		name               string
		featureFlagEnabled bool
		labels             map[string]string
		expectedFailure    bool
	}{
		{
			name:               "flag off, so nothing is gated",
			featureFlagEnabled: false,
			labels:             nil,
		},
		{
			name:               "migrated resources are writable",
			featureFlagEnabled: true,
			labels: map[string]string{
				common.NuclioLabelKeyMigrationFunctionAuth: common.NuclioLabelValueMigrationApplied,
			},
		},
		{
			name:               "unmigrated resources are rejected",
			featureFlagEnabled: true,
			labels:             map[string]string{},
			expectedFailure:    true,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.abstractPlatform.Config.Authentication.FunctionAuthenticationEnabled = testCase.featureFlagEnabled

			err := suite.platform.checkNotMigratedToFunctionAuthentication("Function", "func", testCase.labels)
			if !testCase.expectedFailure {
				suite.Require().NoError(err)
				return
			}

			suite.Require().Error(err)
			suite.Require().Equal(http.StatusPreconditionFailed,
				common.ResolveErrorStatusCodeOrDefault(err, http.StatusInternalServerError))
		})
	}
}

// TestUpdateAPIGatewayIsGatedWhileUnmigrated asserts an unmigrated gateway is rejected before anything is
// written, so a user edit cannot race the migration sweep.
func (suite *AuthMigrationKubePlatformTestSuite) TestUpdateAPIGatewayIsGatedWhileUnmigrated() {
	suite.withFunctionAuthentication(auth.AuthenticationModeAPI)

	apiGateway := suite.compileAPIGateway("gw", "func", auth.AuthenticationModeIguazio, nil)
	suite.nuclioAPIGatewayInterfaceMock.
		On("Get", suite.ctx, "gw", metav1.GetOptions{}).
		Return(apiGateway, nil).
		Once()

	err := suite.platform.UpdateAPIGateway(suite.ctx, &platform.UpdateAPIGatewayOptions{
		APIGatewayConfig: &platform.APIGatewayConfig{
			Meta: platform.APIGatewayMeta{Name: "gw", Namespace: suite.Namespace},
			Spec: apiGateway.Spec,
		},
	})

	suite.Require().Error(err)
	suite.Require().Equal(http.StatusPreconditionFailed,
		common.ResolveErrorStatusCodeOrDefault(err, http.StatusInternalServerError))

	// nothing beyond the initial Get is mocked, so reaching the OPA query or the write would have failed
	suite.nuclioAPIGatewayInterfaceMock.AssertNotCalled(suite.T(), "Update",
		mock.Anything, mock.Anything, mock.Anything)
	suite.nuclioAPIGatewayInterfaceMock.AssertExpectations(suite.T())
}

// TestUpdateAPIGatewayKeepsMigrationLabel asserts an update whose request omits the migration label does
// not erase it, since the update replaces the whole label map and the gate would then reject every later
// edit of a migrated gateway.
func (suite *AuthMigrationKubePlatformTestSuite) TestUpdateAPIGatewayKeepsMigrationLabel() {
	suite.withFunctionAuthentication(auth.AuthenticationModeAPI)

	storedAPIGateway := suite.compileAPIGateway("gw", "func", "", nil)
	markMigratedToFunctionAuthentication(storedAPIGateway)
	suite.nuclioAPIGatewayInterfaceMock.
		On("Get", suite.ctx, "gw", metav1.GetOptions{}).
		Return(storedAPIGateway, nil).
		Once()

	suite.mockedOpaClient.
		On("QueryPermissions",
			mock.AnythingOfType("string"),
			opaclient.ActionUpdate,
			mock.AnythingOfType("*opaclient.PermissionOptions")).
		Return(true, nil).
		Once()

	// the upstream function does not exist, which the update tolerates
	suite.nuclioFunctionInterfaceMock.
		On("Get", suite.ctx, "func", metav1.GetOptions{}).
		Return(nil, &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}).
		Once()

	updatedAPIGateway := suite.expectAPIGatewayUpdate()

	// as a real client sends it: user labels only, no internal migration label
	err := suite.platform.UpdateAPIGateway(suite.ctx, &platform.UpdateAPIGatewayOptions{
		APIGatewayConfig: &platform.APIGatewayConfig{
			Meta: platform.APIGatewayMeta{
				Name:      "gw",
				Namespace: suite.Namespace,
				Labels: map[string]string{
					common.NuclioResourceLabelKeyProjectName: suite.projectName,
				},
			},
			Spec: storedAPIGateway.Spec,
		},
	})
	suite.Require().NoError(err)

	suite.Require().Equal(common.NuclioLabelValueMigrationApplied,
		updatedAPIGateway.Labels[common.NuclioLabelKeyMigrationFunctionAuth])
	suite.nuclioAPIGatewayInterfaceMock.AssertExpectations(suite.T())
}

// TestCreateAPIGatewayStampsMigrationLabel asserts a gateway created while the feature is already on is
// marked as migrated, so it is never gated by a sweep that has already finished.
func (suite *AuthMigrationKubePlatformTestSuite) TestCreateAPIGatewayStampsMigrationLabel() {
	suite.withFunctionAuthentication(auth.AuthenticationModeAPI)

	suite.mockedOpaClient.
		On("QueryPermissions",
			mock.AnythingOfType("string"),
			opaclient.ActionCreate,
			mock.AnythingOfType("*opaclient.PermissionOptions")).
		Return(true, nil).
		Once()

	// the upstream function does not exist, which the create tolerates
	suite.nuclioFunctionInterfaceMock.
		On("Get", suite.ctx, "func", metav1.GetOptions{}).
		Return(nil, &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}).
		Once()

	createdAPIGateway := &v1beta1.NuclioAPIGateway{}
	suite.nuclioAPIGatewayInterfaceMock.
		On("Create", suite.ctx, mock.MatchedBy(func(apiGateway *v1beta1.NuclioAPIGateway) bool {
			*createdAPIGateway = *apiGateway
			return true
		}), metav1.CreateOptions{}).
		Return(createdAPIGateway, nil).
		Once()

	apiGateway := suite.compileAPIGateway("gw", "func", "", nil)
	err := suite.platform.CreateAPIGateway(suite.ctx, &platform.CreateAPIGatewayOptions{
		APIGatewayConfig: &platform.APIGatewayConfig{
			Meta: platform.APIGatewayMeta{
				Name:      "gw",
				Namespace: suite.Namespace,
				Labels: map[string]string{
					common.NuclioResourceLabelKeyProjectName: suite.projectName,
				},
			},
			Spec: apiGateway.Spec,
		},
	})
	suite.Require().NoError(err)

	suite.Require().Equal(common.NuclioLabelValueMigrationApplied,
		createdAPIGateway.Labels[common.NuclioLabelKeyMigrationFunctionAuth])
	suite.nuclioAPIGatewayInterfaceMock.AssertExpectations(suite.T())
}

// TestPreserveMigrationLabel asserts the migration label survives an update request that does not carry it,
// since it is internal bookkeeping and not part of the client-facing API.
func (suite *AuthMigrationKubePlatformTestSuite) TestPreserveMigrationLabel() {
	stored := map[string]string{
		common.NuclioLabelKeyMigrationFunctionAuth: common.NuclioLabelValueMigrationApplied,
		common.NuclioResourceLabelKeyProjectName:   suite.projectName,
	}

	preserved := preserveMigrationLabel(map[string]string{"user-label": "value"}, stored)
	suite.Require().Equal(common.NuclioLabelValueMigrationApplied,
		preserved[common.NuclioLabelKeyMigrationFunctionAuth])
	suite.Require().Equal("value", preserved["user-label"])

	// nothing stored, nothing invented
	suite.Require().NotContains(preserveMigrationLabel(nil, map[string]string{}),
		common.NuclioLabelKeyMigrationFunctionAuth)
}

//
// test helpers
//

// withFunctionAuthentication turns the feature flag on, so the sweep is not a no-op. SetupTest starts every
// test from an empty authentication configuration, and TearDownTest restores the suite-wide one.
func (suite *AuthMigrationKubePlatformTestSuite) withFunctionAuthentication(defaultMode auth.AuthenticationMode) {
	suite.abstractPlatform.Config.Authentication.FunctionAuthenticationEnabled = true
	suite.abstractPlatform.Config.Authentication.DefaultMode = defaultMode
}

func (suite *AuthMigrationKubePlatformTestSuite) compileFunction(name string,
	state functionconfig.FunctionState,
	authenticationMode auth.AuthenticationMode) *v1beta1.NuclioFunction {

	attributes := map[string]interface{}{}
	if authenticationMode != "" {
		attributes[auth.AttributeAuthenticationMode] = string(authenticationMode)
	}

	return &v1beta1.NuclioFunction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: suite.Namespace,
			Labels: map[string]string{
				common.NuclioResourceLabelKeyProjectName: suite.projectName,
			},
		},
		Spec: functionconfig.Spec{
			Triggers: map[string]functionconfig.Trigger{
				"http": {Kind: "http", Name: "http", Attributes: attributes},
			},
		},
		Status: functionconfig.Status{State: state},
	}
}

func (suite *AuthMigrationKubePlatformTestSuite) compileAPIGateway(name, functionName string,
	authenticationMode auth.AuthenticationMode,
	authentication *platform.APIGatewayAuthenticationSpec) *v1beta1.NuclioAPIGateway {

	return &v1beta1.NuclioAPIGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: suite.Namespace,
			Labels: map[string]string{
				common.NuclioResourceLabelKeyProjectName: suite.projectName,
			},
		},
		Spec: platform.APIGatewaySpec{
			Name:               name,
			Host:               name + "-host",
			AuthenticationMode: authenticationMode,
			Authentication:     authentication,
			Upstreams: []platform.APIGatewayUpstreamSpec{
				{
					Kind:           platform.APIGatewayUpstreamKindNuclioFunction,
					NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{Name: functionName},
				},
			},
		},
	}
}

func (suite *AuthMigrationKubePlatformTestSuite) compileBasicAuthentication(username,
	password string) *platform.APIGatewayAuthenticationSpec {
	return &platform.APIGatewayAuthenticationSpec{
		BasicAuth: &platform.BasicAuth{Username: username, Password: password},
	}
}

func (suite *AuthMigrationKubePlatformTestSuite) compileDexAuthentication(
	redirect bool) *platform.APIGatewayAuthenticationSpec {
	return &platform.APIGatewayAuthenticationSpec{
		DexAuth: &ingress.DexAuth{RedirectUnauthorizedToSignIn: redirect},
	}
}

// expectListUnmigrated mocks both of the sweep's list calls, asserting each one filters on the migration
// label rather than listing everything.
func (suite *AuthMigrationKubePlatformTestSuite) expectListUnmigrated(functions []v1beta1.NuclioFunction,
	apiGateways []v1beta1.NuclioAPIGateway) {

	unmigratedSelector := fmt.Sprintf("!%s", common.NuclioLabelKeyMigrationFunctionAuth)

	suite.nuclioFunctionInterfaceMock.
		On("List", suite.ctx, metav1.ListOptions{LabelSelector: unmigratedSelector}).
		Return(&v1beta1.NuclioFunctionList{Items: functions}, nil).
		Once()
	suite.nuclioAPIGatewayInterfaceMock.
		On("List", suite.ctx, metav1.ListOptions{LabelSelector: unmigratedSelector}).
		Return(&v1beta1.NuclioAPIGatewayList{Items: apiGateways}, nil).
		Once()
}

// expectAPIGatewayUpdate captures the api gateway the sweep writes, so the test can assert on it.
func (suite *AuthMigrationKubePlatformTestSuite) expectAPIGatewayUpdate() *v1beta1.NuclioAPIGateway {
	captured := &v1beta1.NuclioAPIGateway{}
	suite.nuclioAPIGatewayInterfaceMock.
		On("Update", suite.ctx, mock.MatchedBy(func(apiGateway *v1beta1.NuclioAPIGateway) bool {
			*captured = *apiGateway
			return true
		}), metav1.UpdateOptions{}).
		Return(captured, nil).
		Once()
	return captured
}

func TestAuthMigrationKubePlatformTestSuite(t *testing.T) {
	suite.Run(t, new(AuthMigrationKubePlatformTestSuite))
}
