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

package kube

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/auth/iguazio"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/client"
	"github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/mocks"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	mockplatform "github.com/nuclio/nuclio/pkg/platform/mock"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/google/go-cmp/cmp"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/cache"
	"k8s.io/client-go/kubernetes/fake"
)

type KubePlatformTestSuite struct {
	suite.Suite
	nuclioioV1beta1InterfaceMock     *mocks.NuclioV1beta1Interface
	nuclioFunctionInterfaceMock      *mocks.NuclioFunctionInterface
	nuclioProjectInterfaceMock       *mocks.NuclioProjectInterface
	nuclioFunctionEventInterfaceMock *mocks.NuclioFunctionEventInterface
	nuclioAPIGatewayInterfaceMock    *mocks.NuclioAPIGatewayInterface
	nuclioioInterfaceMock            *mocks.Interface
	kubeClientSet                    fake.Clientset
	abstractPlatform                 *abstract.Platform
	Namespace                        string
	Logger                           logger.Logger
	mockedPlatform                   *mockplatform.Platform
	platform                         *Platform
	platformKubeConfig               *platformconfig.PlatformKubeConfig
	mockedOpaClient                  *opa.MockClient
	opaOverrideHeaderValue           string
	ctx                              context.Context
}

func (suite *KubePlatformTestSuite) SetupSuite() {
	var err error

	common.SetVersionFromEnv()

	suite.Namespace = "default-namespace"
	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Logger should create successfully")

	suite.ctx = context.Background()

	suite.platformKubeConfig = &platformconfig.PlatformKubeConfig{
		DefaultServiceType: v1.ServiceTypeClusterIP,
	}
	suite.mockedPlatform = &mockplatform.Platform{}
	abstractPlatform, err := abstract.NewPlatform(suite.Logger, suite.mockedPlatform, &platformconfig.Config{
		Kube: *suite.platformKubeConfig,
	}, "")
	suite.Require().NoError(err, "Could not create platform")

	abstractPlatform.ContainerBuilder, err = containerimagebuilderpusher.NewNop(suite.Logger, nil)
	suite.Require().NoError(err)
	suite.abstractPlatform = abstractPlatform
	suite.opaOverrideHeaderValue = "some-dummy-opa-override-value"
	suite.mockedOpaClient = &opa.MockClient{}
	suite.abstractPlatform.OpaClient = suite.mockedOpaClient
	suite.abstractPlatform.ExternalIPAddresses = []string{
		"external-ip",
	}
	suite.kubeClientSet = *fake.NewSimpleClientset()
}

func (suite *KubePlatformTestSuite) SetupTest() {
	suite.ResetCRDMocks()
}

func (suite *KubePlatformTestSuite) ResetCRDMocks() {
	suite.nuclioioInterfaceMock = &mocks.Interface{}
	suite.nuclioioV1beta1InterfaceMock = &mocks.NuclioV1beta1Interface{}
	suite.nuclioFunctionInterfaceMock = &mocks.NuclioFunctionInterface{}
	suite.nuclioProjectInterfaceMock = &mocks.NuclioProjectInterface{}
	suite.nuclioFunctionEventInterfaceMock = &mocks.NuclioFunctionEventInterface{}
	suite.nuclioAPIGatewayInterfaceMock = &mocks.NuclioAPIGatewayInterface{}

	suite.nuclioioInterfaceMock.
		On("NuclioV1beta1").
		Return(suite.nuclioioV1beta1InterfaceMock)
	suite.nuclioioV1beta1InterfaceMock.
		On("NuclioFunctions", suite.Namespace).
		Return(suite.nuclioFunctionInterfaceMock)
	suite.nuclioioV1beta1InterfaceMock.
		On("NuclioProjects", suite.Namespace).
		Return(suite.nuclioProjectInterfaceMock)
	suite.nuclioioV1beta1InterfaceMock.
		On("NuclioFunctionEvents", suite.Namespace).
		Return(suite.nuclioFunctionEventInterfaceMock)
	suite.nuclioioV1beta1InterfaceMock.
		On("NuclioAPIGateways", suite.Namespace).
		Return(suite.nuclioAPIGatewayInterfaceMock)

	getter, err := client.NewGetter(suite.Logger, suite.platform)
	suite.Require().NoError(err)

	suite.platform = &Platform{
		Platform: suite.abstractPlatform,
		getter:   getter,
		consumer: &client.Consumer{
			NuclioClientSet: suite.nuclioioInterfaceMock,
			KubeClientSet:   &suite.kubeClientSet,
		},
		projectsCache: cache.NewExpiring(),
	}
	suite.platform.updater, _ = client.NewUpdater(suite.Logger, suite.platform.consumer, suite.platform)
	suite.platform.deleter, _ = client.NewDeleter(suite.Logger, suite.platform)
	suite.platform.deployer, _ = client.NewDeployer(suite.Logger, suite.platform.consumer, suite.platform)
	suite.platform.projectsClient, _ = NewProjectsClient(suite.platform, suite.abstractPlatform.Config)
}

type ProjectKubePlatformTestSuite struct {
	KubePlatformTestSuite
}

func (suite *ProjectKubePlatformTestSuite) TestGetProjectsCache() {
	suite.nuclioProjectInterfaceMock.On("Create",
		suite.ctx,
		mock.Anything,
		mock.Anything).
		Return(&nuclioio.NuclioProject{}, nil).
		Once()
	defer suite.nuclioProjectInterfaceMock.AssertExpectations(suite.T())

	// create project
	err := suite.platform.CreateProject(suite.ctx, &platform.CreateProjectOptions{
		AuthSession: &auth.NopSession{},
		ProjectConfig: &platform.ProjectConfig{
			Meta: platform.ProjectMeta{
				Name:      "some-name",
				Namespace: suite.Namespace,
			},
		},
	})
	suite.Require().NoError(err)

	// project has been cached
	suite.Require().NotZero(suite.platform.projectsCache.Len())

	// make sure "recently created" is being filtered (returns false) and other project
	// that have been created long time ago is returned
	suite.mockedOpaClient.
		On("QueryPermissionsMultiResources",
			suite.ctx,
			[]string{
				fmt.Sprintf("/projects/%s", "some-name"),
				fmt.Sprintf("/projects/%s", "other-name"),
			},
			opa.ActionRead,
			mock.Anything).
		Return([]bool{false, true}, nil).
		Once()
	defer suite.mockedOpaClient.AssertExpectations(suite.T())

	// return both projects from k8s
	suite.nuclioProjectInterfaceMock.On("List",
		suite.ctx,
		mock.Anything,
		mock.Anything).
		Return(&v1beta1.NuclioProjectList{
			Items: []nuclioio.NuclioProject{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: suite.Namespace,
						Name:      "some-name",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: suite.Namespace,
						Name:      "other-name",
					},
				},
			},
		}, nil).
		Once()

	projects, err := suite.platform.GetProjects(suite.ctx, &platform.GetProjectsOptions{
		AuthSession: &auth.NopSession{},
		Meta: platform.ProjectMeta{
			Namespace: suite.Namespace,
		},
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           []string{"id1"},
			RaiseForbidden:      false,
			OverrideHeaderValue: suite.opaOverrideHeaderValue,
		},
	})
	suite.Require().NoError(err)
	suite.Require().Len(projects, 2, "Expected both projects to returned")

	// delete project, make sure cache revocation works
	suite.nuclioProjectInterfaceMock.On("Delete",
		suite.ctx,
		mock.Anything,
		mock.Anything).
		Return(nil).
		Once()
	err = suite.platform.DeleteProject(suite.ctx, &platform.DeleteProjectOptions{
		AuthSession: &auth.NopSession{},
		Meta: platform.ProjectMeta{
			Name:      "some-name",
			Namespace: suite.Namespace,
		},
	})
	suite.Require().NoError(err)
	suite.Require().Equal(suite.platform.projectsCache.Len(),
		0,
		"project was not removed from cache")
}

type FunctionKubePlatformTestSuite struct {
	KubePlatformTestSuite
}

func (suite *FunctionKubePlatformTestSuite) TestValidateServiceType() {
	for idx, testCase := range []struct {
		name                 string
		serviceType          v1.ServiceType
		shouldFailValidation bool
	}{

		// happy flows
		{
			name:        "empty",
			serviceType: "",
		},
		{
			name:        "nodePort",
			serviceType: v1.ServiceTypeNodePort,
		},
		{
			name:        "clusterIP",
			serviceType: v1.ServiceTypeClusterIP,
		},

		// bad flows
		{
			name:                 "notSupportedLoadBalancer",
			serviceType:          v1.ServiceTypeLoadBalancer,
			shouldFailValidation: true,
		},
		{
			name:                 "notSupportedExternalName",
			serviceType:          v1.ServiceTypeExternalName,
			shouldFailValidation: true,
		},
		{
			name:                 "notSupportedInvalid",
			serviceType:          "blabla",
			shouldFailValidation: true,
		},
	} {
		suite.Run(testCase.name, func() {
			suite.mockedPlatform.
				On("GetProjects", suite.ctx, &platform.GetProjectsOptions{
					Meta: platform.ProjectMeta{
						Name:      platform.DefaultProjectName,
						Namespace: "default",
					},
				}).
				Return([]platform.Project{
					&platform.AbstractProject{},
				}, nil).
				Once()

			// name it with index and shift with 65 to get A as first letter
			functionName := string(rune(idx + 65))
			functionConfig := *functionconfig.NewConfig()
			functionConfig.Spec.ServiceType = testCase.serviceType

			createFunctionOptions := &platform.CreateFunctionOptions{
				Logger:         suite.Logger,
				FunctionConfig: functionConfig,
			}
			createFunctionOptions.FunctionConfig.Meta.Name = functionName
			createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{
				common.NuclioResourceLabelKeyProjectName: platform.DefaultProjectName,
			}
			suite.Logger.DebugWith("Checking function ", "functionName", functionName)

			err := suite.platform.ValidateFunctionConfig(suite.ctx, &createFunctionOptions.FunctionConfig)
			if testCase.shouldFailValidation {
				suite.Require().Error(err, "Validation passed unexpectedly")
			} else {
				suite.Require().NoError(err, "Validation failed unexpectedly")
			}
		})
	}
}

func (suite *FunctionKubePlatformTestSuite) TestEnrichNodeSelector() {
	for _, testCase := range []struct {
		name                         string
		functionNodeSelector         map[string]string
		platformNodeSelector         map[string]string
		projectNodeSelector          map[string]string
		expectedFunctionNodeSelector map[string]string
	}{
		{
			name:                         "all-selectors-empty",
			expectedFunctionNodeSelector: nil,
		},
		{
			name:                         "get-selector-from-platform",
			platformNodeSelector:         map[string]string{"test": "test"},
			expectedFunctionNodeSelector: map[string]string{"test": "test"},
		},
		{
			name: "get-selector-from-project",
			platformNodeSelector: map[string]string{
				"test":  "from-platform",
				"test2": "from-platform2",
			},
			projectNodeSelector: map[string]string{
				"test":  "from-project",
				"test1": "from-project1",
			},
			expectedFunctionNodeSelector: map[string]string{
				"test":  "from-project",
				"test1": "from-project1",
				"test2": "from-platform2",
			},
		},
		{
			name: "get-selector-from-project",
			platformNodeSelector: map[string]string{
				"test":  "from-platform",
				"test2": "from-platform2",
			},
			projectNodeSelector: map[string]string{
				"test":  "from-project",
				"test1": "from-project1",
			},
			functionNodeSelector: map[string]string{"test": "from-function"},
			expectedFunctionNodeSelector: map[string]string{
				"test":  "from-function",
				"test1": "from-project1",
				"test2": "from-platform2",
			},
		},
	} {
		suite.Run(testCase.name, func() {
			suite.mockedPlatform.
				On("GetProjects", suite.ctx, &platform.GetProjectsOptions{
					Meta: platform.ProjectMeta{
						Name:      platform.DefaultProjectName,
						Namespace: suite.Namespace,
					},
				}).
				Return([]platform.Project{
					&platform.AbstractProject{ProjectConfig: platform.ProjectConfig{Spec: platform.ProjectSpec{DefaultFunctionNodeSelector: testCase.projectNodeSelector}}},
				}, nil).
				Once()
			functionConfig := *functionconfig.NewConfig()
			functionConfig.Spec.NodeSelector = testCase.functionNodeSelector

			functionConfig.Meta.Name = testCase.name
			functionConfig.Meta.Namespace = suite.Namespace
			functionConfig.Meta.Labels = map[string]string{
				common.NuclioResourceLabelKeyProjectName: platform.DefaultProjectName,
			}
			suite.platform.Config.Kube.DefaultFunctionNodeSelector = testCase.platformNodeSelector
			suite.Logger.DebugWith("Checking function ", "functionName", testCase.name)

			err := suite.platform.enrichFunctionNodeSelector(suite.ctx, &functionConfig)
			suite.Require().NoError(err, "Validation failed unexpectedly")
			suite.Require().Equal(testCase.expectedFunctionNodeSelector, functionConfig.Spec.NodeSelector)
		})
	}
}

func (suite *FunctionKubePlatformTestSuite) TestFunctionTriggersEnrichmentAndValidation() {

	// return empty api gateways list on enrichFunctionsWithAPIGateways (not tested here)
	suite.nuclioAPIGatewayInterfaceMock.
		On("List", suite.ctx, metav1.ListOptions{}).
		Return(&v1beta1.NuclioAPIGatewayList{}, nil)

	for idx, testCase := range []struct {
		name                     string
		setUpFunction            func() error
		tearDownFunction         func() error
		triggers                 map[string]functionconfig.Trigger
		expectedEnrichedTriggers map[string]functionconfig.Trigger

		// keep empty when no error is expected
		validationError string
	}{
		{
			name:     "EnrichWithDefaultHTTPTrigger",
			triggers: nil,
			expectedEnrichedTriggers: func() map[string]functionconfig.Trigger {
				defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
				defaultHTTPTrigger.Attributes = map[string]interface{}{
					"serviceType": suite.platformKubeConfig.DefaultServiceType,
				}
				return map[string]functionconfig.Trigger{
					defaultHTTPTrigger.Name: defaultHTTPTrigger,
				}
			}(),
		},
		{
			name: "PathIsAvailable",
			setUpFunction: func() error {
				suite.kubeClientSet = *fake.NewSimpleClientset(&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-name",
						Namespace: suite.Namespace,
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "host-and-path-already-in-use.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path: "used-path/",
											},
										},
									},
								},
							},
						},
					},
				})
				return nil
			},
			tearDownFunction: func() error {
				suite.kubeClientSet = *fake.NewSimpleClientset()
				return nil
			},
			triggers: map[string]functionconfig.Trigger{
				"http-with-ingress": {
					Kind: "http",
					Attributes: map[string]interface{}{
						"ingresses": map[string]interface{}{
							"0": map[string]interface{}{
								"host":  "host-and-path-already-in-use.com",
								"paths": []string{"/unused-path"},
							},
						},
					},
				},
			},
		},
		{
			name: "FailPathInUse",
			setUpFunction: func() error {
				suite.kubeClientSet = *fake.NewSimpleClientset(&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-name",
						Namespace: suite.Namespace,
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "host-and-path-already-in-use.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path: "used-path/",
											},
										},
									},
								},
							},
						},
					},
				})
				return nil
			},
			tearDownFunction: func() error {
				suite.kubeClientSet = *fake.NewSimpleClientset()
				return nil
			},
			triggers: map[string]functionconfig.Trigger{
				"http-with-ingress": {
					Kind: "http",
					Attributes: map[string]interface{}{
						"ingresses": map[string]interface{}{
							"0": map[string]interface{}{
								"host":  "host-and-path-already-in-use.com",
								"paths": []string{"/unused-path", "used-path/"},
							},
						},
					},
				},
			},
			validationError: platform.ErrIngressHostPathInUse.Error(),
		},
	} {
		suite.Run(testCase.name, func() {

			// run test case specific set up function if given
			if testCase.setUpFunction != nil {
				err := testCase.setUpFunction()
				suite.Require().NoError(err)
			}

			// mock get projects
			suite.mockedPlatform.On("GetProjects", suite.ctx, &platform.GetProjectsOptions{
				Meta: platform.ProjectMeta{
					Name:      platform.DefaultProjectName,
					Namespace: suite.Namespace,
				},
			}).Return([]platform.Project{
				&platform.AbstractProject{},
			}, nil).Twice()
			// name it with index and shift with 65 to get A as first letter
			functionName := string(rune(idx + 65))
			functionConfig := *functionconfig.NewConfig()

			createFunctionOptions := &platform.CreateFunctionOptions{
				Logger:         suite.Logger,
				FunctionConfig: functionConfig,
			}
			createFunctionOptions.FunctionConfig.Meta.Name = functionName
			createFunctionOptions.FunctionConfig.Meta.Namespace = suite.Namespace
			createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{
				common.NuclioResourceLabelKeyProjectName: platform.DefaultProjectName,
			}
			createFunctionOptions.FunctionConfig.Spec.Triggers = testCase.triggers
			suite.Logger.DebugWith("Enriching and validating function", "functionName", functionName)

			// run enrichment
			err := suite.platform.EnrichFunctionConfig(suite.ctx, &createFunctionOptions.FunctionConfig)
			suite.Require().NoError(err, "Failed to enrich function")

			if testCase.expectedEnrichedTriggers != nil {
				suite.Equal(testCase.expectedEnrichedTriggers,
					createFunctionOptions.FunctionConfig.Spec.Triggers)
			}

			// run validation
			err = suite.platform.ValidateFunctionConfig(suite.ctx, &createFunctionOptions.FunctionConfig)
			if testCase.validationError != "" {
				suite.Require().Error(err, "Validation passed unexpectedly")
				suite.Require().Equal(testCase.validationError, errors.RootCause(err).Error())
			} else {
				suite.Require().NoError(err, "Validation failed unexpectedly")
			}

			// run test case specific tear down function if given
			if testCase.tearDownFunction != nil {
				err := testCase.tearDownFunction()
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *FunctionKubePlatformTestSuite) TestGetFunctionInstanceAndConfig() {
	for _, testCase := range []struct {
		name                    string
		functionName            string
		hasAPIGateways          bool
		apiGateWayName          string
		expectValidationFailure bool
		functionExists          bool
	}{
		{
			name:         "functionNotFound",
			functionName: "not-found",
		},
		{
			name:           "functionFound",
			functionName:   "found-me",
			functionExists: true,
		},
		{
			name:           "functionFoundEnrichedWithAPIGateways",
			functionName:   "found-me",
			hasAPIGateways: true,
			apiGateWayName: "api-gw-name",
			functionExists: true,
		},
	} {
		suite.Run(testCase.name, func() {
			var getFunctionResponseErr error
			var getFunctionResponse *v1beta1.NuclioFunction
			listAPIGatewayResponse := v1beta1.NuclioAPIGatewayList{Items: []v1beta1.NuclioAPIGateway{}}

			// prepare mock responses
			if !testCase.functionExists {
				getFunctionResponseErr = apierrors.NewNotFound(schema.GroupResource{}, "asd")
			} else {
				getFunctionResponse = &v1beta1.NuclioFunction{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: suite.Namespace,
						Name:      testCase.functionName,
					},
				}
			}

			if testCase.hasAPIGateways {
				listAPIGatewayResponse.Items = append(listAPIGatewayResponse.Items,
					v1beta1.NuclioAPIGateway{
						ObjectMeta: metav1.ObjectMeta{
							Name: testCase.apiGateWayName,
						},
						Spec: platform.APIGatewaySpec{
							Upstreams: []platform.APIGatewayUpstreamSpec{
								{
									Kind: platform.APIGatewayUpstreamKindNuclioFunction,
									NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
										Name: testCase.functionName,
									},
								},
							},
						},
					})
			}

			suite.nuclioFunctionInterfaceMock.
				On("Get", suite.ctx, testCase.functionName, metav1.GetOptions{}).
				Return(getFunctionResponse, getFunctionResponseErr).
				Once()
			defer suite.nuclioAPIGatewayInterfaceMock.AssertExpectations(suite.T())

			if testCase.functionExists {
				suite.nuclioAPIGatewayInterfaceMock.
					On("List", suite.ctx, metav1.ListOptions{}).
					Return(&listAPIGatewayResponse, nil).
					Once()
				defer suite.nuclioAPIGatewayInterfaceMock.AssertExpectations(suite.T())
			}

			functionInstance, functionConfigAndStatus, err := suite.platform.
				getFunctionInstanceAndConfig(suite.ctx,
					&platform.GetFunctionsOptions{
						Name:                  testCase.functionName,
						Namespace:             suite.Namespace,
						EnrichWithAPIGateways: true,
					})

			if testCase.expectValidationFailure {
				suite.Require().Error(err)
				return
			}

			// no error
			suite.Require().NoError(err)

			// response might be nil, if function was not found
			if !testCase.functionExists {
				suite.Require().Nil(functionInstance)
				suite.Require().Nil(functionConfigAndStatus)
				return
			}

			// ensure found function matches its function name input
			suite.Require().Equal(functionInstance.Name, testCase.functionName)
			suite.Require().Equal(functionConfigAndStatus.Meta.Name, testCase.functionName)
			suite.Require().Empty(cmp.Diff(functionInstance.Spec, functionConfigAndStatus.Spec))
			suite.Require().Empty(cmp.Diff(functionInstance.Status, functionConfigAndStatus.Status))

			if testCase.hasAPIGateways {
				suite.Require().Contains(functionConfigAndStatus.Status.APIGateways, testCase.apiGateWayName)
			}
		})
	}
}

func (suite *FunctionKubePlatformTestSuite) TestGetFunctionsPermissions() {
	var getFunctionResponse *v1beta1.NuclioFunction

	for _, testCase := range []struct {
		name           string
		opaResponse    bool
		givenMemberIds bool
		raiseForbidden bool
	}{
		{
			name:           "Allowed",
			opaResponse:    true,
			givenMemberIds: true,
			raiseForbidden: true,
		},
		{
			name:           "Forbidden with Error",
			opaResponse:    false,
			givenMemberIds: true,
			raiseForbidden: true,
		},
		{
			name:           "Forbidden no Error",
			opaResponse:    false,
			givenMemberIds: true,
			raiseForbidden: false,
		},
		{
			name:           "No OPA",
			opaResponse:    false,
			givenMemberIds: false,
			raiseForbidden: false,
		},
	} {
		suite.Run(testCase.name, func() {
			var memberIds []string

			functionName := "func"
			projectName := "proj"

			getFunctionResponse = &v1beta1.NuclioFunction{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: suite.Namespace,
					Name:      functionName,
					Labels: map[string]string{
						common.NuclioResourceLabelKeyProjectName: projectName,
					},
				},
			}

			suite.nuclioFunctionInterfaceMock.
				On("Get", suite.ctx, functionName, metav1.GetOptions{}).
				Return(getFunctionResponse, nil).
				Once()
			defer suite.nuclioFunctionInterfaceMock.AssertExpectations(suite.T())

			if testCase.givenMemberIds {
				memberIds = []string{"id1", "id2"}

				suite.mockedOpaClient.
					On("QueryPermissionsMultiResources",
						suite.ctx,
						[]string{fmt.Sprintf("/projects/%s/functions/%s", projectName, functionName)},
						opa.ActionRead,
						&opa.PermissionOptions{
							MemberIds:           memberIds,
							RaiseForbidden:      testCase.raiseForbidden,
							OverrideHeaderValue: suite.opaOverrideHeaderValue,
						}).
					Return([]bool{testCase.opaResponse}, nil).
					Once()
				defer suite.mockedOpaClient.AssertExpectations(suite.T())
			}
			functions, err := suite.platform.GetFunctions(suite.ctx, &platform.GetFunctionsOptions{
				Name:      functionName,
				Namespace: suite.Namespace,
				PermissionOptions: opa.PermissionOptions{
					MemberIds:           memberIds,
					RaiseForbidden:      testCase.raiseForbidden,
					OverrideHeaderValue: suite.opaOverrideHeaderValue,
				},
			})

			if !testCase.opaResponse && testCase.givenMemberIds {
				if testCase.raiseForbidden {
					suite.Require().Error(err)
				} else {
					suite.Require().NoError(err)
					suite.Require().Equal(0, len(functions))
				}
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(1, len(functions))
				suite.Require().Equal(functionName, functions[0].GetConfig().Meta.Name)
			}
		})
	}
}

func (suite *FunctionKubePlatformTestSuite) TestUpdateFunctionPermissions() {
	var getFunctionResponse *v1beta1.NuclioFunction

	for _, testCase := range []struct {
		name        string
		opaResponse bool
	}{
		{
			name:        "Allowed",
			opaResponse: true,
		},
		{
			name:        "Forbidden",
			opaResponse: false,
		},
	} {
		suite.Run(testCase.name, func() {
			functionName := "func"
			projectName := "proj"
			memberIds := []string{"id1", "id2"}

			getFunctionResponse = &v1beta1.NuclioFunction{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: suite.Namespace,
					Name:      functionName,
					Labels: map[string]string{
						common.NuclioResourceLabelKeyProjectName: projectName,
					},
				},
				Status: functionconfig.Status{
					State: functionconfig.FunctionStateReady,
				},
			}

			suite.nuclioFunctionInterfaceMock.
				On("Get", suite.ctx, functionName, metav1.GetOptions{}).
				Return(getFunctionResponse, nil).
				Once()
			defer suite.nuclioFunctionInterfaceMock.AssertExpectations(suite.T())

			suite.mockedOpaClient.
				On("QueryPermissions",
					fmt.Sprintf("/projects/%s/functions/%s", projectName, functionName),
					opa.ActionUpdate,
					&opa.PermissionOptions{
						MemberIds:           memberIds,
						RaiseForbidden:      true,
						OverrideHeaderValue: suite.opaOverrideHeaderValue,
					}).
				Return(testCase.opaResponse, nil).
				Once()
			defer suite.mockedOpaClient.AssertExpectations(suite.T())

			if testCase.opaResponse {

				// verify
				verifyUpdateFunction := func(function *v1beta1.NuclioFunction) bool {
					suite.Require().Equal(functionName, function.Name)
					suite.Require().Equal(suite.Namespace, function.Namespace)

					return true
				}

				suite.nuclioFunctionInterfaceMock.
					On("Get", suite.ctx, functionName, metav1.GetOptions{}).
					Return(getFunctionResponse, nil).
					Once()

				suite.nuclioFunctionInterfaceMock.
					On("Update",
						suite.ctx,
						mock.MatchedBy(verifyUpdateFunction),
						metav1.UpdateOptions{}).
					Return(getFunctionResponse, nil).
					Once()
				defer suite.nuclioFunctionInterfaceMock.AssertExpectations(suite.T())
			}

			err := suite.platform.UpdateFunction(suite.ctx, &platform.UpdateFunctionOptions{
				FunctionMeta: &functionconfig.Meta{
					Name:      functionName,
					Namespace: suite.Namespace,
				},
				PermissionOptions: opa.PermissionOptions{
					MemberIds:           memberIds,
					OverrideHeaderValue: suite.opaOverrideHeaderValue,
				},
			})

			if !testCase.opaResponse {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *FunctionKubePlatformTestSuite) TestDeleteFunctionPermissions() {
	for _, testCase := range []struct {
		name        string
		opaResponse bool
	}{
		{
			name:        "Allowed",
			opaResponse: true,
		},
		{
			name:        "Forbidden",
			opaResponse: false,
		},
	} {
		suite.Run(testCase.name, func() {
			functionName := "func"
			projectName := "proj"
			memberIds := []string{"id1", "id2"}

			returnedFunction := platform.AbstractFunction{
				Config: functionconfig.Config{
					Meta: functionconfig.Meta{
						Namespace: suite.Namespace,
						Name:      functionName,
						Labels: map[string]string{
							common.NuclioResourceLabelKeyProjectName: projectName,
						},
					},
				},
			}

			suite.mockedPlatform.
				On("GetFunctions", suite.ctx, &platform.GetFunctionsOptions{
					Name:      functionName,
					Namespace: suite.Namespace,
					PermissionOptions: opa.PermissionOptions{
						MemberIds:           memberIds,
						OverrideHeaderValue: suite.opaOverrideHeaderValue,
					},
				}).
				Return([]platform.Function{&returnedFunction}, nil).
				Once()
			defer suite.mockedPlatform.AssertExpectations(suite.T())

			suite.mockedOpaClient.
				On("QueryPermissions",
					fmt.Sprintf("/projects/%s/functions/%s", projectName, functionName),
					opa.ActionDelete,
					&opa.PermissionOptions{
						MemberIds:           memberIds,
						RaiseForbidden:      true,
						OverrideHeaderValue: suite.opaOverrideHeaderValue,
					}).
				Return(testCase.opaResponse, nil).
				Once()
			defer suite.mockedOpaClient.AssertExpectations(suite.T())

			if testCase.opaResponse {
				suite.nuclioAPIGatewayInterfaceMock.
					On("List", suite.ctx, metav1.ListOptions{}).
					Return(&v1beta1.NuclioAPIGatewayList{}, nil).
					Once()
				defer suite.nuclioAPIGatewayInterfaceMock.AssertExpectations(suite.T())

				suite.nuclioFunctionInterfaceMock.
					On("Delete", suite.ctx, functionName, metav1.DeleteOptions{}).
					Return(nil).
					Once()
				defer suite.nuclioFunctionInterfaceMock.AssertExpectations(suite.T())
			}

			err := suite.platform.DeleteFunction(suite.ctx, &platform.DeleteFunctionOptions{
				FunctionConfig: functionconfig.Config{
					Meta: functionconfig.Meta{
						Name:      functionName,
						Namespace: suite.Namespace,
					},
				},
				PermissionOptions: opa.PermissionOptions{
					MemberIds:           memberIds,
					OverrideHeaderValue: suite.opaOverrideHeaderValue,
				},
				IgnoreFunctionStateValidation: true,
			})

			if !testCase.opaResponse {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *FunctionKubePlatformTestSuite) TestRenderFunctionIngress() {
	suite.platform.Config.Kube.DefaultHTTPIngressHostTemplate = "{{ .ResourceName }}.{{ .Namespace }}.test.com"

	for _, testCase := range []struct {
		name        string
		httpTrigger functionconfig.Trigger
		want        map[string]interface{}
	}{
		{
			name: "nothingBackground",
			httpTrigger: functionconfig.Trigger{
				Name: "http-trigger",
				Kind: "http",
				Attributes: map[string]interface{}{
					"ingresses": map[string]interface{}{},
				},
			},
			want: map[string]interface{}{},
		},
		{
			name: "fromCustom",
			httpTrigger: functionconfig.Trigger{
				Name: "http-trigger",
				Kind: "http",
				Attributes: map[string]interface{}{
					"ingresses": map[string]interface{}{
						"0": map[string]interface{}{
							"hostTemplate": "{{ .ResourceName }}.custom.test.com",
						},
					},
				},
			},
			want: map[string]interface{}{
				"0": map[string]interface{}{
					"hostTemplate": "{{ .ResourceName }}.custom.test.com",
					"host":         "some-name.custom.test.com",
					"pathType":     networkingv1.PathTypeImplementationSpecific,
				},
			},
		},
		{
			name: "fromDefault",
			httpTrigger: functionconfig.Trigger{
				Name: "http-trigger",
				Kind: "http",
				Attributes: map[string]interface{}{
					"ingresses": map[string]interface{}{
						"0": map[string]interface{}{
							"hostTemplate": "@nuclio.fromDefault",
						},
					},
				},
			},
			want: map[string]interface{}{
				"0": map[string]interface{}{
					"hostTemplate": "@nuclio.fromDefault",
					"host":         "some-name.some-namespace.test.com",
					"pathType":     networkingv1.PathTypeImplementationSpecific,
				},
			},
		},
		{
			name: "multipleIngresses",
			httpTrigger: functionconfig.Trigger{
				Name: "http-trigger",
				Kind: "http",
				Attributes: map[string]interface{}{
					"ingresses": map[string]interface{}{
						"0": map[string]interface{}{
							"hostTemplate": "@nuclio.fromDefault",
						},
						"1": map[string]interface{}{
							"host": "leave-it-as.is.com",
						},
					},
				},
			},
			want: map[string]interface{}{
				"0": map[string]interface{}{
					"hostTemplate": "@nuclio.fromDefault",
					"host":         "some-name.some-namespace.test.com",
					"pathType":     networkingv1.PathTypeImplementationSpecific,
				},
				"1": map[string]interface{}{
					"host":     "leave-it-as.is.com",
					"pathType": networkingv1.PathTypeImplementationSpecific,
				},
			},
		},
		{
			name: "fromDefaultHostEmpty",
			httpTrigger: functionconfig.Trigger{
				Name: "http-trigger",
				Kind: "http",
				Attributes: map[string]interface{}{
					"ingresses": map[string]interface{}{
						"0": map[string]interface{}{
							"hostTemplate": "@nuclio.fromDefault",
							"host":         "",
						},
					},
				},
			},
			want: map[string]interface{}{
				"0": map[string]interface{}{
					"hostTemplate": "@nuclio.fromDefault",
					"host":         "some-name.some-namespace.test.com",
					"pathType":     networkingv1.PathTypeImplementationSpecific,
				},
			},
		},
		{
			name: "fromDefaultNoHostOverriding",
			httpTrigger: functionconfig.Trigger{
				Name: "http-trigger",
				Kind: "http",
				Attributes: map[string]interface{}{
					"ingresses": map[string]interface{}{
						"0": map[string]interface{}{
							"hostTemplate": "@nuclio.fromDefault",
							"host":         "dont-override-me.com",
						},
					},
				},
			},
			want: map[string]interface{}{
				"0": map[string]interface{}{
					"hostTemplate": "@nuclio.fromDefault",
					"host":         "dont-override-me.com",
					"pathType":     networkingv1.PathTypeImplementationSpecific,
				},
			},
		},
	} {
		suite.Run(testCase.name, func() {
			functionConfig := &functionconfig.Config{
				Meta: functionconfig.Meta{
					Name:      "some-name",
					Namespace: "some-namespace",
				},
				Spec: functionconfig.Spec{
					Triggers: map[string]functionconfig.Trigger{
						testCase.httpTrigger.Name: testCase.httpTrigger,
					},
				},
			}
			err := suite.platform.enrichHTTPTriggers(suite.ctx, functionConfig)
			suite.Require().NoError(err)
			suite.Require().Equal(testCase.want,
				functionConfig.Spec.Triggers[testCase.httpTrigger.Name].Attributes["ingresses"])

		})
	}

}

func (suite *FunctionKubePlatformTestSuite) TestAlignIngressHostSubdomain() {
	type args struct {
		host              string
		randomCharsLength int
	}

	for _, testCase := range []struct {
		name string
		args args
		want *regexp.Regexp
	}{
		{
			name: "noChange",
			args: args{host: "something.simple.com", randomCharsLength: 5},
			want: regexp.MustCompile("something.simple.com"),
		},
		{
			name: "truncate",
			args: args{host: func() string {
				longSubdomainLevel := "this-is-a-very-long-level-and-should-be-truncated-by-the-random-chars-length"
				return fmt.Sprintf("%s.blabla.com", longSubdomainLevel)
			}(), randomCharsLength: 5},
			want: regexp.MustCompile("this-is-a-very-long-level-and-should-be-truncated-by-the-[a-z0-9]{5}.blabla.com"),
		},
	} {
		suite.Run(testCase.name, func() {
			alignedIngressHostSubdomain := suite.platform.alignIngressHostSubdomainLevel(testCase.args.host, testCase.args.randomCharsLength)
			suite.Require().Regexp(testCase.want, alignedIngressHostSubdomain)
		})
	}
}

func (suite *FunctionKubePlatformTestSuite) TestEnrichFunctionWithPreemptionSpec() {
	preemptibleNodes := &platformconfig.PreemptibleNodes{
		NodeSelector: map[string]string{
			"node-label-key": "node-label-value",
		},
		Tolerations: []v1.Toleration{
			{
				Key:   "toleration-key",
				Value: "toleration-value",
			},
		},
	}
	suite.platform.Config.Kube.PreemptibleNodes = preemptibleNodes
	functionName := "some-func"
	functionConfig := functionconfig.NewConfig()
	functionConfig.Meta.Name = functionName
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesAllow

	// "" -> Allow - add tolerations
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Equal(preemptibleNodes.Tolerations, functionConfig.Spec.Tolerations)

	// Allow -> Allow - make sure tolertions were added once
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Equal(functionConfig.Spec.Tolerations, preemptibleNodes.Tolerations)

	// Allow -> Constrain - make sure it preserves tolerations and add node selectors
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesConstrain
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Equal(functionConfig.Spec.Tolerations, preemptibleNodes.Tolerations)
	suite.Require().Equal(
		functionConfig.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
		preemptibleNodes.CompileAffinityByLabelSelectorScheduleOnOneOfMatchingNodes())

	// Constrain -> Constrain - make sure spec was added once
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Equal(functionConfig.Spec.Tolerations, preemptibleNodes.Tolerations)
	suite.Require().Equal(
		functionConfig.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
		preemptibleNodes.CompileAffinityByLabelSelectorScheduleOnOneOfMatchingNodes())

	// Constrain -> Allow - make sure node selectors were removed
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesAllow
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Equal(functionConfig.Spec.Tolerations, preemptibleNodes.Tolerations)
	for key := range preemptibleNodes.NodeSelector {
		_, ok := functionConfig.Spec.NodeSelector[key]
		suite.Require().False(ok)
	}

	// Allow -> Allow - make sure custom labels are not pruned
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesAllow
	customFunctionLabels := map[string]string{
		"customkey": "customvalue",
	}
	functionConfig.Spec.NodeSelector = customFunctionLabels

	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Equal(functionConfig.Spec.Tolerations, preemptibleNodes.Tolerations)
	suite.Require().Equal(customFunctionLabels, functionConfig.Spec.NodeSelector)

	// Allow -> Constrain - merge custom labels with preemption labels
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesConstrain
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Equal(functionConfig.Spec.Tolerations, preemptibleNodes.Tolerations)
	suite.Require().Equal(customFunctionLabels, functionConfig.Spec.NodeSelector)
	suite.Require().Equal(
		functionConfig.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
		preemptibleNodes.CompileAffinityByLabelSelectorScheduleOnOneOfMatchingNodes())

	// Constrain -> Allow - make sure allow does not prune custom labels + constrain label selectors
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesAllow
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Equal(functionConfig.Spec.Tolerations, preemptibleNodes.Tolerations)
	suite.Require().Equal(customFunctionLabels, functionConfig.Spec.NodeSelector)

	// clean node selector
	functionConfig.Spec.NodeSelector = nil

	// "" -> Allow - add custom tolerations
	customTolerations := v1.Toleration{
		Key:   "custom-toleration-key",
		Value: "custom-toleration-value",
	}
	functionConfig.Spec.Tolerations = []v1.Toleration{customTolerations}
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesAllow
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().NotEmpty(cmp.Diff([]v1.Toleration{
		{
			Key:   preemptibleNodes.Tolerations[0].Key,
			Value: preemptibleNodes.Tolerations[0].Value,
		},
		customTolerations,
	}, functionConfig.Spec.Tolerations))

	// Allow -> Constrain - preserve custom tolerations
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesConstrain
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().NotEmpty(cmp.Diff([]v1.Toleration{
		{
			Key:   preemptibleNodes.Tolerations[0].Key,
			Value: preemptibleNodes.Tolerations[0].Value,
		},
		{
			Key:   "custom-toleration-key",
			Value: "custom-toleration-value",
		},
	}, functionConfig.Spec.Tolerations))

	// * END OF ALLOW <> CONSTRAIN * //
	functionConfig.Spec.NodeSelector = nil
	functionConfig.Spec.Affinity = nil
	functionConfig.Spec.Tolerations = nil

	// add constrain spec
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesConstrain
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesPrevent
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Empty(functionConfig.Spec.NodeSelector)
	suite.Require().Empty(functionConfig.Spec.Tolerations)

	// affinity is pruned (prevention is done using taints)
	suite.Require().Nil(functionConfig.Spec.Affinity)

	// prevention is done using node label selectors
	preemptibleNodes = &platformconfig.PreemptibleNodes{
		NodeSelector: map[string]string{
			"node-label-key":   "node-label-value",
			"node-label-key-2": "node-label-value-2",
		},
	}
	suite.platform.Config.Kube.PreemptibleNodes = preemptibleNodes
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Empty(cmp.Diff(
		functionConfig.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
		preemptibleNodes.CompileAntiAffinityByLabelSelectorNoScheduleOnMatchingNodes()))

	// Prevent -> Constrain - pruned anti affinity + add affinity
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesConstrain
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Empty(cmp.Diff(
		functionConfig.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
		preemptibleNodes.CompileAffinityByLabelSelectorScheduleOnOneOfMatchingNodes()))

	// Constrain -> Prevent - read anti affinity
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesPrevent
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Empty(functionConfig.Spec.NodeSelector)
	suite.Require().Empty(functionConfig.Spec.Tolerations)
	suite.Require().Empty(cmp.Diff(
		functionConfig.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
		preemptibleNodes.CompileAntiAffinityByLabelSelectorNoScheduleOnMatchingNodes()))

	// Prevent -> Allow
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesAllow
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Empty(functionConfig.Spec.Tolerations) // no toleration were given on config, that's intentional
	suite.Require().Nil(functionConfig.Spec.Affinity)

	// preserve custom affinity
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesPrevent
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	functionConfig.Spec.Affinity.PodAffinity = &v1.PodAffinity{}
	functionConfig.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = []v1.PreferredSchedulingTerm{}
	functionConfig.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = append(
		functionConfig.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
		v1.NodeSelectorTerm{
			MatchFields: []v1.NodeSelectorRequirement{
				{
					Key: "dummy",
				},
			},
		},
	)
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesAllow
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().NotNil(functionConfig.Spec.Affinity.PodAffinity)
	suite.Require().NotNil(functionConfig.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution)
	suite.Require().Equal("dummy",
		functionConfig.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchFields[0].Key)

	// reset specs, constrain -> prevent (for tolerations + node selector)
	preemptibleNodes = &platformconfig.PreemptibleNodes{
		NodeSelector: map[string]string{
			"a": "c",
			"b": "d",
		},
		Tolerations: []v1.Toleration{
			{
				Key:      "some-key",
				Value:    "some-value",
				Operator: v1.TolerationOpEqual,
				Effect:   v1.TaintEffectNoSchedule,
			},
		},
	}
	suite.platform.Config.Kube.PreemptibleNodes = preemptibleNodes
	functionConfig = functionconfig.NewConfig()
	functionConfig.Meta.Name = functionName
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesConstrain

	// no affinity
	suite.Require().Nil(functionConfig.Spec.Affinity)
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)

	// enriched affinity
	suite.Require().NotNil(functionConfig.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)

	// constrain -> prevent
	functionConfig.Spec.PreemptionMode = functionconfig.RunOnPreemptibleNodesPrevent
	suite.platform.enrichFunctionPreemptionSpec(suite.ctx, preemptibleNodes, functionConfig)
	suite.Require().Nil(functionConfig.Spec.Affinity)

}

func (suite *FunctionKubePlatformTestSuite) TestEnrichFunctionWithUserNameLabel() {

	functionName := "some-func"
	functionConfig := *functionconfig.NewConfig()
	authSession := &auth.IguazioSession{
		Username: "some-user",
	}

	// inject auth session to context
	ctx := context.WithValue(suite.ctx, auth.AuthSessionContextKey, authSession)
	suite.mockedPlatform.
		On("GetProjects", ctx, &platform.GetProjectsOptions{
			Meta: platform.ProjectMeta{
				Name:      platform.DefaultProjectName,
				Namespace: suite.Namespace,
			},
		}).
		Return([]platform.Project{
			&platform.AbstractProject{},
		}, nil).
		Once()

	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger:         suite.Logger,
		FunctionConfig: functionConfig,
		AuthSession:    authSession,
	}
	createFunctionOptions.FunctionConfig.Meta.Name = functionName
	createFunctionOptions.FunctionConfig.Meta.Namespace = suite.Namespace
	createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{
		common.NuclioResourceLabelKeyProjectName: platform.DefaultProjectName,
	}

	suite.Logger.DebugWith("Enriching function", "functionName", functionName)

	err := suite.platform.EnrichFunctionConfig(ctx, &createFunctionOptions.FunctionConfig)
	suite.Require().NoError(err)

	suite.Require().Equal("some-user", createFunctionOptions.FunctionConfig.Meta.Labels[iguazio.IguazioUsernameLabel])
}

func (suite *FunctionKubePlatformTestSuite) TestUsernameLabelsEnrichment() {
	for _, testCase := range []struct {
		name                  string
		fullUsername          string
		expectedUsernameLabel string
		expectedDomainLabel   string
	}{
		{
			name:                  "with-name-and-domain",
			fullUsername:          "foo@bar.com",
			expectedUsernameLabel: "foo",
			expectedDomainLabel:   "bar.com",
		},
		{
			name:                  "with-only-name",
			fullUsername:          "foo",
			expectedUsernameLabel: "foo",
		},
		{
			name: "empty",
		},
		// test cases to check that we don't panic on wrong usernames
		{
			name:                  "wrong-with-two-ats",
			fullUsername:          "foo@bar@test",
			expectedUsernameLabel: "foo",
			expectedDomainLabel:   "bar",
		},
		{
			name:                  "wrong-with-empty-domain",
			fullUsername:          "foo@",
			expectedUsernameLabel: "foo",
		},
		{
			name:                "wrong-with-empty-name",
			fullUsername:        "@bar",
			expectedDomainLabel: "bar",
		},
	} {
		suite.Run(testCase.name, func() {
			testContext := context.WithValue(suite.ctx,
				auth.AuthSessionContextKey,
				&auth.IguazioSession{
					Username: testCase.fullUsername,
				},
			)
			labels := make(map[string]string)
			suite.platform.EnrichLabels(testContext, labels)

			usernameLabel, _ := labels[iguazio.IguazioUsernameLabel]
			suite.Require().Equal(testCase.expectedUsernameLabel, usernameLabel)

			domainLabel, _ := labels[iguazio.IguazioDomainLabel]
			suite.Require().Equal(testCase.expectedDomainLabel, domainLabel)
		})
	}
}

type FunctionEventKubePlatformTestSuite struct {
	KubePlatformTestSuite
}

func (suite *FunctionEventKubePlatformTestSuite) TestGetFunctionEventsPermissions() {
	for _, testCase := range []struct {
		name           string
		opaResponse    bool
		givenMemberIds bool
		raiseForbidden bool
	}{
		{
			name:           "Allowed",
			opaResponse:    true,
			givenMemberIds: true,
			raiseForbidden: true,
		},
		{
			name:           "Forbidden with Error",
			opaResponse:    false,
			givenMemberIds: true,
			raiseForbidden: true,
		},
		{
			name:           "Forbidden no Error",
			opaResponse:    false,
			givenMemberIds: true,
			raiseForbidden: false,
		},
		{
			name:           "No OPA",
			opaResponse:    false,
			givenMemberIds: false,
			raiseForbidden: false,
		},
	} {
		suite.Run(testCase.name, func() {
			var memberIds []string

			functionEventName := "funcEvent"
			functionName := "func"
			projectName := "proj"

			getFunctionEventResponse := &v1beta1.NuclioFunctionEvent{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: suite.Namespace,
					Name:      functionEventName,
					Labels: map[string]string{
						common.NuclioResourceLabelKeyFunctionName: functionName,
						common.NuclioResourceLabelKeyProjectName:  projectName,
					},
				},
			}

			suite.nuclioFunctionEventInterfaceMock.
				On("Get", suite.ctx, functionEventName, metav1.GetOptions{}).
				Return(getFunctionEventResponse, nil).
				Once()
			defer suite.nuclioFunctionEventInterfaceMock.AssertExpectations(suite.T())

			if testCase.givenMemberIds {
				memberIds = []string{"id1", "id2"}
				suite.mockedOpaClient.
					On("QueryPermissionsMultiResources",
						suite.ctx,
						[]string{fmt.Sprintf("/projects/%s/functions/%s/function-events/%s",
							projectName,
							functionName,
							functionEventName)},
						opa.ActionRead,
						&opa.PermissionOptions{
							MemberIds:           memberIds,
							RaiseForbidden:      testCase.raiseForbidden,
							OverrideHeaderValue: suite.opaOverrideHeaderValue,
						}).
					Return([]bool{testCase.opaResponse}, nil).
					Once()
				defer suite.mockedOpaClient.AssertExpectations(suite.T())
			}
			functionEvents, err := suite.platform.GetFunctionEvents(suite.ctx, &platform.GetFunctionEventsOptions{
				Meta: platform.FunctionEventMeta{
					Name:      functionEventName,
					Namespace: suite.Namespace,
				},
				PermissionOptions: opa.PermissionOptions{
					MemberIds:           memberIds,
					RaiseForbidden:      testCase.raiseForbidden,
					OverrideHeaderValue: suite.opaOverrideHeaderValue,
				},
			})

			if !testCase.opaResponse && testCase.givenMemberIds {
				if testCase.raiseForbidden {
					suite.Require().Error(err)
				} else {
					suite.Require().NoError(err)
					suite.Require().Equal(0, len(functionEvents))
				}
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(1, len(functionEvents))
				suite.Require().Equal(functionEventName, functionEvents[0].GetConfig().Meta.Name)
			}
		})
	}
}

func (suite *FunctionEventKubePlatformTestSuite) TestUpdateFunctionEventPermissions() {
	for _, testCase := range []struct {
		name        string
		opaResponse bool
	}{
		{
			name:        "Allowed",
			opaResponse: true,
		},
		{
			name:        "Forbidden",
			opaResponse: false,
		},
	} {
		suite.Run(testCase.name, func() {
			functionEventName := "funcEvent"
			functionName := "func"
			projectName := "proj"
			memberIds := []string{"id1", "id2"}

			getFunctionEventResponse := &v1beta1.NuclioFunctionEvent{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: suite.Namespace,
					Name:      functionEventName,
					Labels: map[string]string{
						common.NuclioResourceLabelKeyFunctionName: functionName,
						common.NuclioResourceLabelKeyProjectName:  projectName,
					},
				},
			}

			suite.nuclioFunctionEventInterfaceMock.
				On("Get", suite.ctx, functionEventName, metav1.GetOptions{}).
				Return(getFunctionEventResponse, nil).
				Once()
			defer suite.nuclioFunctionEventInterfaceMock.AssertExpectations(suite.T())

			suite.mockedOpaClient.
				On("QueryPermissions",
					fmt.Sprintf("/projects/%s/functions/%s/function-events/%s",
						projectName,
						functionName,
						functionEventName),
					opa.ActionUpdate,
					&opa.PermissionOptions{
						MemberIds:           memberIds,
						RaiseForbidden:      true,
						OverrideHeaderValue: suite.opaOverrideHeaderValue,
					}).
				Return(testCase.opaResponse, nil).
				Once()
			defer suite.mockedOpaClient.AssertExpectations(suite.T())

			if testCase.opaResponse {

				// verify
				verifyUpdateFunctionEvent := func(functionEvent *v1beta1.NuclioFunctionEvent) bool {
					suite.Require().Equal(functionEventName, functionEvent.Name)
					suite.Require().Equal(suite.Namespace, functionEvent.Namespace)

					return true
				}

				suite.nuclioFunctionEventInterfaceMock.
					On("Update",
						suite.ctx,
						mock.MatchedBy(verifyUpdateFunctionEvent),
						metav1.UpdateOptions{}).
					Return(getFunctionEventResponse, nil).
					Once()

				defer suite.nuclioFunctionEventInterfaceMock.AssertExpectations(suite.T())
			}

			err := suite.platform.UpdateFunctionEvent(suite.ctx, &platform.UpdateFunctionEventOptions{
				FunctionEventConfig: platform.FunctionEventConfig{
					Meta: platform.FunctionEventMeta{
						Name:      functionEventName,
						Namespace: suite.Namespace,
						Labels: map[string]string{
							common.NuclioResourceLabelKeyFunctionName: functionName,
							common.NuclioResourceLabelKeyProjectName:  functionName,
						},
					},
					Spec: platform.FunctionEventSpec{},
				},
				PermissionOptions: opa.PermissionOptions{
					MemberIds:           memberIds,
					OverrideHeaderValue: suite.opaOverrideHeaderValue,
				},
			})

			if !testCase.opaResponse {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *FunctionEventKubePlatformTestSuite) TestDeleteFunctionEventPermissions() {
	for _, testCase := range []struct {
		name        string
		opaResponse bool
	}{
		{
			name:        "Allowed",
			opaResponse: true,
		},
		{
			name:        "Forbidden",
			opaResponse: false,
		},
	} {
		suite.Run(testCase.name, func() {
			functionEventName := "funcEvent"
			functionName := "func"
			projectName := "proj"
			memberIds := []string{"id1", "id2"}

			getFunctionEventResponse := &v1beta1.NuclioFunctionEvent{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: suite.Namespace,
					Name:      functionEventName,
					Labels: map[string]string{
						common.NuclioResourceLabelKeyFunctionName: functionName,
						common.NuclioResourceLabelKeyProjectName:  projectName,
					},
				},
			}

			suite.nuclioFunctionEventInterfaceMock.
				On("Get", suite.ctx, functionEventName, metav1.GetOptions{}).
				Return(getFunctionEventResponse, nil).
				Once()
			defer suite.nuclioFunctionEventInterfaceMock.AssertExpectations(suite.T())

			suite.mockedOpaClient.
				On("QueryPermissions",
					fmt.Sprintf("/projects/%s/functions/%s/function-events/%s",
						projectName,
						functionName,
						functionEventName),
					opa.ActionDelete,
					&opa.PermissionOptions{
						MemberIds:           memberIds,
						RaiseForbidden:      true,
						OverrideHeaderValue: suite.opaOverrideHeaderValue,
					}).
				Return(testCase.opaResponse, nil).
				Once()
			defer suite.mockedOpaClient.AssertExpectations(suite.T())

			if testCase.opaResponse {

				suite.nuclioFunctionEventInterfaceMock.
					On("Delete", suite.ctx, functionEventName, metav1.DeleteOptions{}).
					Return(nil).
					Once()
				defer suite.nuclioFunctionEventInterfaceMock.AssertExpectations(suite.T())
			}

			err := suite.platform.DeleteFunctionEvent(suite.ctx, &platform.DeleteFunctionEventOptions{
				Meta: platform.FunctionEventMeta{
					Name:      functionEventName,
					Namespace: suite.Namespace,
				},
				PermissionOptions: opa.PermissionOptions{
					MemberIds:           memberIds,
					OverrideHeaderValue: suite.opaOverrideHeaderValue,
				},
			})

			if !testCase.opaResponse {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

type APIGatewayKubePlatformTestSuite struct {
	KubePlatformTestSuite
}

func (suite *APIGatewayKubePlatformTestSuite) TestAPIGatewayEnrichmentAndValidation() {

	// return empty api gateways list on enrichFunctionsWithAPIGateways (not tested here)
	suite.nuclioAPIGatewayInterfaceMock.
		On("List", suite.ctx, metav1.ListOptions{}).
		Return(&v1beta1.NuclioAPIGatewayList{}, nil)

	for _, testCase := range []struct {
		name             string
		setUpFunction    func() error
		tearDownFunction func() error
		apiGatewayConfig *platform.APIGatewayConfig

		// keep empty to skip the enrichment validation
		expectedEnrichedAPIGateway *platform.APIGatewayConfig

		// the matching api gateway upstream functions
		upstreamFunctions []*v1beta1.NuclioFunction

		// whether to validate upstream functions existing
		validateFunctionsExistence bool

		// keep empty when shouldn't fail
		validationError string

		// keep empty when not verifying session enrichment
		authSession *auth.IguazioSession
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
			name: "UserNameEnrichedFromSession",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				return &apiGatewayConfig
			}(),
			expectedEnrichedAPIGateway: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Meta.Labels = map[string]string{
					iguazio.IguazioUsernameLabel: "some-username",
				}
				return &apiGatewayConfig
			}(),
			authSession: &auth.IguazioSession{
				Username: "some-username",
			},
		},
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
		{
			name: "ValidateAPIGatewayFunctionHasNoIngresses",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Upstreams[0].NuclioFunction.Name = "function-with-ingresses"
				return &apiGatewayConfig
			}(),
			upstreamFunctions: []*v1beta1.NuclioFunction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "function-with-ingresses",
					},
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
		},
		{
			name: "ValidateAPIGatewayCanaryFunctionHasNoIngresses",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Upstreams[0].NuclioFunction.Name = "function-without-ingresses"
				apiGatewayConfig.Spec.Upstreams = append(apiGatewayConfig.Spec.Upstreams, platform.APIGatewayUpstreamSpec{
					Kind: platform.APIGatewayUpstreamKindNuclioFunction,
					NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
						Name: "function-with-ingresses-2",
					},
				})
				return &apiGatewayConfig
			}(),
			upstreamFunctions: []*v1beta1.NuclioFunction{
				{}, // primary upstream function is empty (has no ingresses)
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "function-with-ingresses-2",
					},
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
		},
		{
			name: "PathIsAvailable",
			setUpFunction: func() error {
				suite.kubeClientSet = *fake.NewSimpleClientset(&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-name",
						Namespace: suite.Namespace,
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "this-host-and-path-are-used.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path: "different-path/",
											},
										},
									},
								},
							},
						},
					},
				})
				return nil
			},
			tearDownFunction: func() error {
				suite.kubeClientSet = *fake.NewSimpleClientset()
				return nil
			},
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Host = "this-host-and-path-are-used.com"
				apiGatewayConfig.Spec.Path = "//same-path"
				return &apiGatewayConfig
			}(),
		},
		{
			name: "FailPathInUse",
			setUpFunction: func() error {
				suite.kubeClientSet = *fake.NewSimpleClientset(&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-name",
						Namespace: suite.Namespace,
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "this-host-and-path-are-used.com",
								IngressRuleValue: networkingv1.IngressRuleValue{
									HTTP: &networkingv1.HTTPIngressRuleValue{
										Paths: []networkingv1.HTTPIngressPath{
											{
												Path: "same-path/",
											},
										},
									},
								},
							},
						},
					},
				})
				return nil
			},
			tearDownFunction: func() error {
				suite.kubeClientSet = *fake.NewSimpleClientset()
				return nil
			},
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				apiGatewayConfig.Spec.Host = "this-host-and-path-are-used.com"
				apiGatewayConfig.Spec.Path = "//same-path"
				return &apiGatewayConfig
			}(),
			validationError: platform.ErrIngressHostPathInUse.Error(),
		},
		{
			name: "ValidateFunctionsExistenceSanity",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				return &apiGatewayConfig
			}(),
			upstreamFunctions: []*v1beta1.NuclioFunction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default-func-name",
					},
				},
			},
			validateFunctionsExistence: true,
		},
		{
			name: "ValidateFunctionsExistenceFailed",
			apiGatewayConfig: func() *platform.APIGatewayConfig {
				apiGatewayConfig := suite.compileAPIGatewayConfig()
				return &apiGatewayConfig
			}(),
			validateFunctionsExistence: true,
			validationError:            "Function default-func-name does not exist",
		},
	} {
		suite.Run(testCase.name, func() {
			defer suite.ResetCRDMocks()
			if testCase.expectedEnrichedAPIGateway != nil {
				if testCase.expectedEnrichedAPIGateway.Meta.Labels == nil {
					testCase.expectedEnrichedAPIGateway.Meta.Labels = map[string]string{}
				}
				if testCase.authSession != nil {
					suite.ctx = context.WithValue(suite.ctx, auth.AuthSessionContextKey, testCase.authSession)
				}
				suite.platform.EnrichLabels(suite.ctx, testCase.expectedEnrichedAPIGateway.Meta.Labels)
			}

			// run test case specific set up function if given
			if testCase.setUpFunction != nil {
				err := testCase.setUpFunction()
				suite.Require().NoError(err)
			}

			// run enrichment
			suite.platform.enrichAPIGatewayConfig(suite.ctx, testCase.apiGatewayConfig, nil)
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
					On("Get", suite.ctx, upstream.NuclioFunction.Name, metav1.GetOptions{}).
					Return(upstreamFunction, getFunctionsError).
					Once()
			}

			// run validation
			err := suite.platform.validateAPIGatewayConfig(suite.ctx, testCase.apiGatewayConfig,
				testCase.validateFunctionsExistence,
				nil)
			if testCase.validationError != "" {
				suite.Require().Error(err)
				suite.Require().Equal(testCase.validationError, errors.RootCause(err).Error())
			} else {
				suite.Require().NoError(err)
			}

			// run test case specific tear down function if given
			if testCase.tearDownFunction != nil {
				err := testCase.tearDownFunction()
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *APIGatewayKubePlatformTestSuite) TestAPIGatewayUpdate() {
	for _, testCase := range []struct {
		name                    string
		updateAPIGatewayOptions func(baseAPIGatewayConfig *platform.APIGatewayConfig) *platform.UpdateAPIGatewayOptions
	}{
		{
			name: "UpdateFields",
			updateAPIGatewayOptions: func(baseAPIGatewayConfig *platform.APIGatewayConfig) *platform.UpdateAPIGatewayOptions {
				updateAPIGatewayOptions := &platform.UpdateAPIGatewayOptions{
					APIGatewayConfig: &platform.APIGatewayConfig{
						Meta:   baseAPIGatewayConfig.Meta,
						Spec:   baseAPIGatewayConfig.Spec,
						Status: baseAPIGatewayConfig.Status,
					},
				}
				// modify a field
				updateAPIGatewayOptions.APIGatewayConfig.Spec.Host = "update-me.com"
				updateAPIGatewayOptions.APIGatewayConfig.Meta.Labels = map[string]string{
					common.NuclioResourceLabelKeyProjectName: "some-test",
					"newLabel":                               "label-value",
				}
				updateAPIGatewayOptions.APIGatewayConfig.Meta.Annotations = map[string]string{
					"newAnnotation": "annotation-value",
				}
				return updateAPIGatewayOptions
			},
		},
	} {
		suite.Run(testCase.name, func() {
			apiGatewayConfig := suite.compileAPIGatewayConfig()
			updateAPIGatewayOptions := testCase.updateAPIGatewayOptions(&apiGatewayConfig)

			// get before update
			suite.nuclioAPIGatewayInterfaceMock.
				On("Get", suite.ctx, apiGatewayConfig.Meta.Name, metav1.GetOptions{}).
				Return(&v1beta1.NuclioAPIGateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      apiGatewayConfig.Meta.Name,
						Namespace: apiGatewayConfig.Meta.Namespace,
						Labels: map[string]string{
							common.NuclioResourceLabelKeyProjectName: "some-test",
						},
					},
					Spec:   apiGatewayConfig.Spec,
					Status: apiGatewayConfig.Status,
				}, nil).
				Once()

			verifyAPIGatewayToUpdate := func(apiGatewayToUpdate *v1beta1.NuclioAPIGateway) bool {
				suite.Require().Empty(cmp.Diff(updateAPIGatewayOptions.APIGatewayConfig.Spec, apiGatewayToUpdate.Spec))
				suite.Require().Empty(cmp.Diff(updateAPIGatewayOptions.APIGatewayConfig.Meta.Annotations, apiGatewayToUpdate.Annotations))
				suite.Require().Empty(cmp.Diff(updateAPIGatewayOptions.APIGatewayConfig.Meta.Labels, apiGatewayToUpdate.Labels))
				suite.Require().Equal(platform.APIGatewayStateWaitingForProvisioning, apiGatewayToUpdate.Status.State)
				return true
			}

			// mock kubernetes update
			suite.nuclioAPIGatewayInterfaceMock.
				On("Update",
					suite.ctx,
					mock.MatchedBy(verifyAPIGatewayToUpdate),
					mock.Anything).
				Return(&v1beta1.NuclioAPIGateway{}, nil).
				Once()

			// no function with matching upstreams
			suite.nuclioFunctionInterfaceMock.
				On("Get",
					suite.ctx,
					apiGatewayConfig.Spec.Upstreams[0].NuclioFunction.Name,
					metav1.GetOptions{}).
				Return(nil,
					&apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}).
				Once()

			// update
			err := suite.platform.UpdateAPIGateway(suite.ctx, updateAPIGatewayOptions)
			suite.Require().NoError(err)
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
					NuclioFunction: &platform.NuclioFunctionAPIGatewaySpec{
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
	suite.Run(t, new(ProjectKubePlatformTestSuite))
	suite.Run(t, new(FunctionKubePlatformTestSuite))
	suite.Run(t, new(FunctionEventKubePlatformTestSuite))
	suite.Run(t, new(APIGatewayKubePlatformTestSuite))
}
