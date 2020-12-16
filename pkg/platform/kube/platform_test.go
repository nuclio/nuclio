package kube

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	"github.com/nuclio/nuclio/pkg/platform/mock"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/google/go-cmp/cmp"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
)

type KubePlatformTestSuite struct {
	suite.Suite
	mockedPlatform     *mock.Platform
	Logger             logger.Logger
	Platform           *Platform
	PlatformKubeConfig *platformconfig.PlatformKubeConfig
}

func (suite *KubePlatformTestSuite) SetupSuite() {
	var err error

	common.SetVersionFromEnv()

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Logger should create successfully")

	suite.PlatformKubeConfig = &platformconfig.PlatformKubeConfig{
		DefaultServiceType: v1.ServiceTypeClusterIP,
	}
	suite.mockedPlatform = &mock.Platform{}
	abstractPlatform, err := abstract.NewPlatform(suite.Logger, suite.mockedPlatform, &platformconfig.Config{
		Kube: *suite.PlatformKubeConfig,
	})
	suite.Require().NoError(err, "Could not create platform")

	abstractPlatform.ContainerBuilder, err = containerimagebuilderpusher.NewNop(suite.Logger, nil)
	suite.Require().NoError(err)

	suite.Platform = &Platform{
		Platform: abstractPlatform,
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

	for _, testCase := range []struct {
		name             string
		apiGatewayConfig *platform.APIGatewayConfig

		// keep empty to skip the enrichment validation
		expectedEnrichedAPIGateway *platform.APIGatewayConfig

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
		{
			name: "NamespaceExistence",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Meta.Namespace = ""
				return &apiGatewayConfig
			}(),
			validationError: "Api gateway namespace must be provided in metadata",
		},
		{
			name: "NameExistence",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Meta.Name = ""
				apiGatewayConfig.Spec.Name = ""
				return &apiGatewayConfig
			}(),
			validationError: "Api gateway name must be provided in metadata",
		},
		{
			name: "MetadataNameEqualSpecName",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Meta.Name = "name1"
				apiGatewayConfig.Spec.Name = "name2"
				return &apiGatewayConfig
			}(),
			validationError: "Api gateway metadata.name must match api gateway spec.name",
		},
		{
			name: "TestReservedNameValidations",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Meta.Name = "dashboard"
				apiGatewayConfig.Spec.Name = "dashboard"
				return &apiGatewayConfig
			}(),
			validationError: "Api gateway name 'dashboard' is reserved and cannot be used",
		},
		{
			name: "MoreThanTwoUpstreams",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				upstream := apiGatewayConfig.Spec.Upstreams[0]
				apiGatewayConfig.Spec.Upstreams = []platform.APIGatewayUpstreamSpec{upstream, upstream, upstream}
				return &apiGatewayConfig
			}(),
			validationError: "Received more than 2 upstreams. Currently not supported",
		},
		{
			name: "UpsteamsIsMissing",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Upstreams = []platform.APIGatewayUpstreamSpec{}
				return &apiGatewayConfig
			}(),
			validationError: "One or more upstreams must be provided in spec",
		},
		{
			name: "HostIsMissing",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Host = ""
				return &apiGatewayConfig
			}(),
			validationError: "Host must be provided in spec",
		},
		{
			name: "BadUpstreamKind",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Upstreams[0].Kind = "bad-kind"
				return &apiGatewayConfig
			}(),
			validationError: "Unsupported upstream kind: 'bad-kind'. (Currently supporting only nucliofunction)",
		},
		{
			name: "AllUpstreamsMustBeOfTheSameKind",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				differentKindUpstream := apiGatewayConfig.Spec.Upstreams[0]
				differentKindUpstream.Kind = "kind-2"
				apiGatewayConfig.Spec.Upstreams = append(apiGatewayConfig.Spec.Upstreams, differentKindUpstream)
				return &apiGatewayConfig
			}(),
			validationError: "All upstreams must be of the same kind",
		},
	} {
		suite.Run(testCase.name, func() {
			// run enrichment
			suite.Platform.EnrichAPIGatewayConfig(testCase.apiGatewayConfig)
			if testCase.expectedEnrichedAPIGateway != nil {
				suite.Require().Empty(cmp.Diff(testCase.expectedEnrichedAPIGateway, testCase.apiGatewayConfig))
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
			Namespace: "default-namespace",
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
