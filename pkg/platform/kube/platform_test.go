package kube

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/mocks"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	mockplatform "github.com/nuclio/nuclio/pkg/platform/mock"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/google/go-cmp/cmp"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubePlatformTestSuite struct {
	suite.Suite
	mockedPlatform                *mockplatform.Platform
	nuclioioV1beta1InterfaceMock  *mocks.NuclioV1beta1Interface
	nuclioFunctionInterfaceMock   *mocks.NuclioFunctionInterface
	nuclioAPIGatewayInterfaceMock *mocks.NuclioAPIGatewayInterface
	nuclioioInterfaceMock         *mocks.Interface
	Namespace                     string
	Logger                        logger.Logger
	Platform                      *Platform
	PlatformKubeConfig            *platformconfig.PlatformKubeConfig
}

func (suite *KubePlatformTestSuite) SetupSuite() {
	var err error

	common.SetVersionFromEnv()

	suite.Namespace = "default-namespace"
	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Logger should create successfully")

	suite.PlatformKubeConfig = &platformconfig.PlatformKubeConfig{
		DefaultServiceType: v1.ServiceTypeClusterIP,
	}
	suite.mockedPlatform = &mockplatform.Platform{}
	abstractPlatform, err := abstract.NewPlatform(suite.Logger, suite.mockedPlatform, &platformconfig.Config{
		Kube: *suite.PlatformKubeConfig,
	})
	suite.Require().NoError(err, "Could not create platform")

	abstractPlatform.ContainerBuilder, err = containerimagebuilderpusher.NewNop(suite.Logger, nil)
	suite.Require().NoError(err)

	// mock nuclioio interface all the way down
	suite.nuclioioInterfaceMock = &mocks.Interface{}
	suite.nuclioioV1beta1InterfaceMock = &mocks.NuclioV1beta1Interface{}
	suite.nuclioFunctionInterfaceMock = &mocks.NuclioFunctionInterface{}
	suite.nuclioAPIGatewayInterfaceMock = &mocks.NuclioAPIGatewayInterface{}

	suite.nuclioioInterfaceMock.
		On("NuclioV1beta1").
		Return(suite.nuclioioV1beta1InterfaceMock)
	suite.nuclioioV1beta1InterfaceMock.
		On("NuclioFunctions", suite.Namespace).
		Return(suite.nuclioFunctionInterfaceMock)
	suite.nuclioioV1beta1InterfaceMock.
		On("NuclioAPIGateways", suite.Namespace).
		Return(suite.nuclioAPIGatewayInterfaceMock)

	consumer := &consumer{
		nuclioClientSet: suite.nuclioioInterfaceMock,
	}

	getter, err := newGetter(suite.Logger, suite.Platform)
	suite.Require().NoError(err)

	suite.Platform = &Platform{
		Platform: abstractPlatform,
		getter:   getter,
		consumer: consumer,
	}
}

type FunctionKubePlatformTestSuite struct {
	KubePlatformTestSuite
}

func (suite *FunctionKubePlatformTestSuite) TestFunctionTriggersEnriched() {
	for idx, testCase := range []struct {
		triggers                 map[string]functionconfig.Trigger
		expectedEnrichedTriggers map[string]functionconfig.Trigger
		shouldFailValidation     bool
	}{

		// enrich with default http trigger
		{
			triggers: nil,
			expectedEnrichedTriggers: func() map[string]functionconfig.Trigger {
				defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
				defaultHTTPTrigger.Attributes = map[string]interface{}{
					"serviceType": suite.PlatformKubeConfig.DefaultServiceType,
				}
				return map[string]functionconfig.Trigger{
					defaultHTTPTrigger.Name: defaultHTTPTrigger,
				}

			}(),
		},
	} {
		suite.mockedPlatform.On("GetProjects", &platform.GetProjectsOptions{
			Meta: platform.ProjectMeta{
				Name:      platform.DefaultProjectName,
				Namespace: suite.Platform.ResolveDefaultNamespace(""),
			},
		}).Return([]platform.Project{
			&platform.AbstractProject{},
		}, nil).Once()

		// name it with index and shift with 65 to get A as first letter
		functionName := string(rune(idx + 65))
		functionConfig := *functionconfig.NewConfig()

		createFunctionOptions := &platform.CreateFunctionOptions{
			Logger:         suite.Logger,
			FunctionConfig: functionConfig,
		}
		createFunctionOptions.FunctionConfig.Meta.Name = functionName
		createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{
			"nuclio.io/project-name": platform.DefaultProjectName,
		}
		createFunctionOptions.FunctionConfig.Spec.Triggers = testCase.triggers
		suite.Logger.DebugWith("Checking function ", "functionName", functionName)

		err := suite.Platform.EnrichFunctionConfig(&createFunctionOptions.FunctionConfig)
		suite.Require().NoError(err, "Failed to enrich function")

		err = suite.Platform.ValidateFunctionConfig(&createFunctionOptions.FunctionConfig)
		if testCase.shouldFailValidation {
			suite.Require().Error(err, "Validation passed unexpectedly")
			continue
		}

		suite.Require().NoError(err, "Validation failed unexpectedly")
		suite.Equal(testCase.expectedEnrichedTriggers,
			createFunctionOptions.FunctionConfig.Spec.Triggers)
	}
}

type APIGatewayKubePlatformTestSuite struct {
	KubePlatformTestSuite
}

func (suite *APIGatewayKubePlatformTestSuite) TestAPIGatewayEnrichmentAndValidation() {

	// return empty api gateways list on enrichFunctionsWithAPIGateways (not tested here)
	suite.nuclioAPIGatewayInterfaceMock.
		On("List", metav1.ListOptions{}).
		Return(&v1beta1.NuclioAPIGatewayList{}, nil)

	for _, testCase := range []struct {
		name             string
		apiGatewayConfig *platform.APIGatewayConfig

		// keep empty to skip the enrichment validation
		expectedEnrichedAPIGateway *platform.APIGatewayConfig

		// the matching api gateway upstream functions
		upstreamFunctions []*v1beta1.NuclioFunction

		// keep empty when shouldn't fail
		validationError string
	}{
		{
			name: "SpecNameEnrichedFromMetaName",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Name = ""
				apiGatewayConfig.Meta.Name = "meta-name"
				return &apiGatewayConfig
			}(),
			expectedEnrichedAPIGateway: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Name = "meta-name"
				apiGatewayConfig.Meta.Name = "meta-name"
				return &apiGatewayConfig
			}(),
		},
		{
			name: "MetaNameEnrichedFromSpecName",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Name = "spec-name"
				apiGatewayConfig.Meta.Name = ""
				return &apiGatewayConfig
			}(),
			expectedEnrichedAPIGateway: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Name = "spec-name"
				apiGatewayConfig.Meta.Name = "spec-name"
				return &apiGatewayConfig
			}(),
		},

		// test enrichment of waiting for provisioning state
		{
			name: "EnrichWaitingForProvisioningState",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Status.State = ""
				return &apiGatewayConfig
			}(),
			expectedEnrichedAPIGateway: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Status.State = platform.APIGatewayStateWaitingForProvisioning
				return &apiGatewayConfig
			}(),
		},

		// test namespace existence
		{
			name: "ValidateNamespaceExistence",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Meta.Namespace = ""
				return &apiGatewayConfig
			}(),
			validationError: "Api gateway namespace must be provided in metadata",
		},
		{
			name: "ValidateNameExistence",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Meta.Name = ""
				apiGatewayConfig.Spec.Name = ""
				return &apiGatewayConfig
			}(),
			validationError: "Api gateway name must be provided in metadata",
		},
		{
			name: "ValidateNameEqualsInSpecAndMeta",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Meta.Name = "name1"
				apiGatewayConfig.Spec.Name = "name2"
				return &apiGatewayConfig
			}(),
			validationError: "Api gateway metadata.name must match api gateway spec.name",
		},
		{
			name: "ValidateNoReservedName",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Meta.Name = "dashboard"
				apiGatewayConfig.Spec.Name = "dashboard"
				return &apiGatewayConfig
			}(),
			validationError: "Api gateway name 'dashboard' is reserved and cannot be used",
		},

		// test len(upstreams) > 2
		{
			name: "ValidateNoMoreThanTwoUpstreams",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				upstream := apiGatewayConfig.Spec.Upstreams[0]
				apiGatewayConfig.Spec.Upstreams = []platform.APIGatewayUpstreamSpec{upstream, upstream, upstream}
				return &apiGatewayConfig
			}(),
			validationError: "Received more than 2 upstreams. Currently not supported",
		},
		{
			name: "ValidateAtLeastOneUpstream",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Upstreams = []platform.APIGatewayUpstreamSpec{}
				return &apiGatewayConfig
			}(),
			validationError: "One or more upstreams must be provided in spec",
		},
		{
			name: "ValidateHostExistence",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Host = ""
				return &apiGatewayConfig
			}(),
			validationError: "Host must be provided in spec",
		},
		{
			name: "ValidateUpstreamKind",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Upstreams[0].Kind = "bad-kind"
				return &apiGatewayConfig
			}(),
			validationError: "Unsupported upstream kind: 'bad-kind'. (Currently supporting only nucliofunction)",
		},

		// test all upstreams have same kind
		{
			name: "ValidateAllUpstreamsHaveSameKind",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				differentKindUpstream := apiGatewayConfig.Spec.Upstreams[0]
				differentKindUpstream.Kind = "kind-2"
				apiGatewayConfig.Spec.Upstreams = append(apiGatewayConfig.Spec.Upstreams, differentKindUpstream)
				return &apiGatewayConfig
			}(),
			validationError: "All upstreams must be of the same kind",
		},

		// test api gateway upstream function has no ingresses
		{
			name: "ValidateAPIGatewayFunctionHasNoIngresses",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Upstreams[0].Nucliofunction.Name = "function-with-ingresses"
				return &apiGatewayConfig
			}(),
			upstreamFunctions: []*v1beta1.NuclioFunction{
				{
					Spec: functionconfig.Spec{
						Triggers: map[string]functionconfig.Trigger{
							"http-with-ingress": {
								Kind: "http",
								Attributes: map[string]interface{}{
									"ingresses": map[string]interface{}{
										"0": map[string]interface{}{
											"host":  "some-host",
											"paths": []string{"/"},
										},
									},
								},
							},
						},
					},
				},
			},
			validationError: "Api gateway upstream function: function-with-ingresses must not have an ingress",
		},

		// test api gateway canary upstream function has no ingresses
		{
			name: "ValidateAPIGatewayCanaryFunctionHasNoIngresses",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Upstreams[0].Nucliofunction.Name = "function-without-ingresses"
				apiGatewayConfig.Spec.Upstreams = append(apiGatewayConfig.Spec.Upstreams, platform.APIGatewayUpstreamSpec{
					Kind: platform.APIGatewayUpstreamKindNuclioFunction,
					Nucliofunction: &platform.NuclioFunctionAPIGatewaySpec{
						Name: "function-with-ingresses-2",
					},
				})
				return &apiGatewayConfig
			}(),
			upstreamFunctions: []*v1beta1.NuclioFunction{
				{}, // primary upstream function is empty (has no ingresses)
				{
					Spec: functionconfig.Spec{
						Triggers: map[string]functionconfig.Trigger{
							"http-with-ingress": {
								Kind: "http",
								Attributes: map[string]interface{}{
									"ingresses": map[string]interface{}{
										"0": map[string]interface{}{
											"host":  "some-host",
											"paths": []string{"/"},
										},
									},
								},
							},
						},
					},
				},
			},
			validationError: "Api gateway upstream function: function-with-ingresses-2 must not have an ingress",
		},
	} {
		suite.Run(testCase.name, func() {

			// run enrichment
			suite.Platform.EnrichAPIGatewayConfig(testCase.apiGatewayConfig)
			if testCase.expectedEnrichedAPIGateway != nil {
				suite.Require().Empty(cmp.Diff(testCase.expectedEnrichedAPIGateway, testCase.apiGatewayConfig))
			}

			// mock Get functions, when iterating over upstreams on validateAPIGatewayFunctionsHaveNoIngresses
			for idx, upstream := range testCase.apiGatewayConfig.Spec.Upstreams {
				var upstreamFunction *v1beta1.NuclioFunction
				var getFunctionsError interface{}

				if len(testCase.upstreamFunctions) > idx {
					upstreamFunction = testCase.upstreamFunctions[idx]
					getFunctionsError = nil
				} else {

					// return no function if not specified (not found)
					upstreamFunction = &v1beta1.NuclioFunction{}
					getFunctionsError = &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
				}

				suite.nuclioFunctionInterfaceMock.
					On("Get", upstream.Nucliofunction.Name, metav1.GetOptions{}).
					Return(upstreamFunction, getFunctionsError).
					Once()
			}

			// run validation
			err := suite.Platform.ValidateAPIGatewayConfig(testCase.apiGatewayConfig)
			if testCase.validationError != "" {
				suite.Assert().Error(err)
				suite.Assert().Equal(testCase.validationError, errors.RootCause(err).Error())
			} else {
				suite.Assert().NoError(err)
			}
		})
	}
}

func (suite *APIGatewayKubePlatformTestSuite) compileAPIGatewayConfig() platform.APIGatewayConfig {
	return platform.APIGatewayConfig{
		Meta: platform.APIGatewayMeta{
			Name:      "default-name",
			Namespace: suite.Namespace,
		},
		Spec: platform.APIGatewaySpec{
			Name:               "default-name",
			Host:               "default-host",
			AuthenticationMode: ingress.AuthenticationModeNone,
			Upstreams: []platform.APIGatewayUpstreamSpec{
				{
					Kind: platform.APIGatewayUpstreamKindNuclioFunction,
					Nucliofunction: &platform.NuclioFunctionAPIGatewaySpec{
						Name: "default-func-name",
					},
				},
			},
		},
		Status: platform.APIGatewayStatus{
			State: platform.APIGatewayStateWaitingForProvisioning,
		},
	}
}

func TestKubePlatformTestSuite(t *testing.T) {
	suite.Run(t, new(FunctionKubePlatformTestSuite))
	suite.Run(t, new(APIGatewayKubePlatformTestSuite))
}
