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
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	kubecommon "github.com/nuclio/nuclio/pkg/platform/kube/common"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeployFunctionTestSuite struct {
	KubeTestSuite
}

func (suite *DeployFunctionTestSuite) TestFailOnMalformedIngressesStructure() {
	createFunctionOptionsMalformedIngressesStructure := suite.CompileCreateFunctionOptions("resource-schema")
	createFunctionOptionsMalformedIngressesStructure.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"http-trigger": {
			Attributes: map[string]interface{}{
				"ingresses": "I should be a map and not a string",
			},
		},
	}

	createFunctionOptionsMalformedSpecificIngressStructure := suite.CompileCreateFunctionOptions("resource-schema")
	createFunctionOptionsMalformedSpecificIngressStructure.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"http-trigger": {
			Attributes: map[string]interface{}{
				"ingresses": map[string]interface{}{
					"0": map[string]interface{}{
						"host": "some-host",
						"paths": []string{"/"},
					},
					"malformed-ingress": "I should be a map and not a string",
				},
			},
		},
	}

	type testStruct struct {
		createFunctionOptions *platform.CreateFunctionOptions
		expectedErrorRootCause         error
	}

	for _, testAttributes := range []testStruct{
		{
			createFunctionOptions: createFunctionOptionsMalformedIngressesStructure,
			expectedErrorRootCause: errors.New("Malformed ingresses format for trigger 'http-trigger' (expects a map)"),
		},
		{
			createFunctionOptions: createFunctionOptionsMalformedSpecificIngressStructure,
			expectedErrorRootCause: errors.New("Malformed structure for ingress 'malformed-ingress' in trigger 'http-trigger'"),
		},
	} {
		suite.DeployFunctionExpectError(testAttributes.createFunctionOptions,
			testAttributes.expectedErrorRootCause,
			func(deployResult *platform.CreateFunctionResult) bool {

			return true
		})
	}
}

func (suite *DeployFunctionTestSuite) TestStaleResourceVersion() {
	var resourceVersion string

	createFunctionOptions := suite.CompileCreateFunctionOptions("resource-schema")

	afterFirstDeploy := func(deployResult *platform.CreateFunctionResult) bool {

		// get the function
		functions, err := suite.Platform.GetFunctions(&platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})
		suite.Require().NoError(err)
		function := functions[0]

		// save resource version
		resourceVersion = function.GetConfig().Meta.ResourceVersion
		suite.Require().NotEmpty(resourceVersion)

		// ensure using newest resource version on second deploy
		createFunctionOptions.FunctionConfig.Meta.ResourceVersion = resourceVersion

		// change source code
		createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(`
def handler(context, event):
  return "darn it!"
`))
		return true
	}

	afterSecondDeploy := func(deployResult *platform.CreateFunctionResult) bool {

		// get the function
		functions, err := suite.Platform.GetFunctions(&platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})
		suite.Require().NoError(err, "Failed to get functions")

		function := functions[0]
		suite.Require().NotEqual(resourceVersion,
			function.GetConfig().Meta.ResourceVersion,
			"Resource version should be changed between deployments")

		// we expect a failure due to a stale resource version
		suite.DeployFunctionExpectError(createFunctionOptions, nil, func(deployResult *platform.CreateFunctionResult) bool {
			suite.Require().Nil(deployResult, "Deployment results is nil when creation failed")
			return true
		})

		return true
	}

	suite.DeployFunctionAndRedeploy(createFunctionOptions, afterFirstDeploy, afterSecondDeploy)
}

func (suite *DeployFunctionTestSuite) TestSecurityContext() {
	runAsUserID := int64(1000)
	runAsGroupID := int64(2000)
	fsGroup := int64(3000)
	functionName := "security-context"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.SecurityContext = &v1.PodSecurityContext{
		RunAsUser:  &runAsUserID,
		RunAsGroup: &runAsGroupID,
		FSGroup:    &fsGroup,
	}
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		deploymentInstance := &appsv1.Deployment{}
		suite.GetResourceAndUnmarshal("deployment",
			kubecommon.DeploymentNameFromFunctionName(functionName),
			deploymentInstance)

		// ensure function deployment was enriched
		suite.Require().NotNil(deploymentInstance.Spec.Template.Spec.SecurityContext.RunAsUser)
		suite.Require().NotNil(deploymentInstance.Spec.Template.Spec.SecurityContext.RunAsGroup)
		suite.Require().NotNil(deploymentInstance.Spec.Template.Spec.SecurityContext.FSGroup)

		// verify deployment spec security context values
		suite.Require().Equal(runAsUserID, *deploymentInstance.Spec.Template.Spec.SecurityContext.RunAsUser)
		suite.Require().Equal(runAsGroupID, *deploymentInstance.Spec.Template.Spec.SecurityContext.RunAsGroup)
		suite.Require().Equal(fsGroup, *deploymentInstance.Spec.Template.Spec.SecurityContext.FSGroup)

		// verify running function indeed using the right uid / gid / groups
		podName := fmt.Sprintf("deployment/%s", kubecommon.DeploymentNameFromFunctionName(functionName))
		results, err := suite.executeKubectl([]string{"exec", podName, "--", "id"}, nil)
		suite.Require().NoError(err, "Failed to execute `id` command on function pod")
		suite.Require().Equal(fmt.Sprintf(`uid=%d gid=%d groups=%d,%d`,
			runAsUserID,
			runAsGroupID,
			runAsGroupID,
			fsGroup),
			strings.TrimSpace(results.Output))
		return true
	})
}

func (suite *DeployFunctionTestSuite) TestAugmentedConfig() {
	runAsUserID := int64(1000)
	runAsGroupID := int64(2000)
	functionAvatar := "demo-avatar"
	functionLabels := map[string]string{
		"my-function": "is-labeled",
	}
	suite.PlatformConfiguration.FunctionAugmentedConfigs = []platformconfig.LabelSelectorAndConfig{
		{
			LabelSelector: metav1.LabelSelector{
				MatchLabels: functionLabels,
			},
			FunctionConfig: functionconfig.Config{
				Spec: functionconfig.Spec{
					Avatar: functionAvatar,
				},
			},
			Kubernetes: platformconfig.Kubernetes{
				Deployment: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: v1.PodTemplateSpec{
							Spec: v1.PodSpec{
								SecurityContext: &v1.PodSecurityContext{
									RunAsUser:  &runAsUserID,
									RunAsGroup: &runAsGroupID,
								},
							},
						},
					},
				},
			},
		},
	}
	functionName := "augmented-config"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Meta.Labels = functionLabels
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		deploymentInstance := &appsv1.Deployment{}
		functionInstance := &nuclioio.NuclioFunction{}
		suite.GetResourceAndUnmarshal("nucliofunction",
			functionName,
			functionInstance)
		suite.GetResourceAndUnmarshal("deployment",
			kubecommon.DeploymentNameFromFunctionName(functionName),
			deploymentInstance)

		// ensure function spec was enriched
		suite.Require().Equal(functionAvatar, functionInstance.Spec.Avatar)

		// ensure function deployment was enriched
		suite.Require().NotNil(deploymentInstance.Spec.Template.Spec.SecurityContext.RunAsUser)
		suite.Require().NotNil(deploymentInstance.Spec.Template.Spec.SecurityContext.RunAsGroup)
		suite.Require().Equal(runAsUserID, *deploymentInstance.Spec.Template.Spec.SecurityContext.RunAsUser)
		suite.Require().Equal(runAsGroupID, *deploymentInstance.Spec.Template.Spec.SecurityContext.RunAsGroup)
		return true
	})
}

func (suite *DeployFunctionTestSuite) TestMinMaxReplicas() {
	functionName := "min-max-replicas"
	two := 2
	three := 3
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.MinReplicas = &two
	createFunctionOptions.FunctionConfig.Spec.MaxReplicas = &three
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		hpaInstance := &autoscalingv1.HorizontalPodAutoscaler{}
		suite.GetResourceAndUnmarshal("hpa", kubecommon.HPANameFromFunctionName(functionName), hpaInstance)
		suite.Require().Equal(two, int(*hpaInstance.Spec.MinReplicas))
		suite.Require().Equal(three, int(hpaInstance.Spec.MaxReplicas))
		return true
	})
}

func (suite *DeployFunctionTestSuite) TestDefaultHTTPTrigger() {
	defaultTriggerFunctionName := "with-default-http-trigger"
	createDefaultTriggerFunctionOptions := suite.CompileCreateFunctionOptions(defaultTriggerFunctionName)
	suite.DeployFunction(createDefaultTriggerFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// ensure only 1 http trigger exists, always.
		suite.ensureTriggerAmount(defaultTriggerFunctionName, "http", 1)
		defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
		return suite.verifyCreatedTrigger(defaultTriggerFunctionName, defaultHTTPTrigger)
	})

	customTriggerFunctionName := "custom-http-trigger"
	createCustomTriggerFunctionOptions := suite.CompileCreateFunctionOptions(customTriggerFunctionName)
	customTrigger := functionconfig.Trigger{
		Kind:       "http",
		Name:       "custom-trigger",
		MaxWorkers: 3,
	}
	createCustomTriggerFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		customTrigger.Name: customTrigger,
	}
	suite.DeployFunction(createCustomTriggerFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// ensure only 1 http trigger exists, always.
		suite.ensureTriggerAmount(customTriggerFunctionName, "http", 1)
		return suite.verifyCreatedTrigger(customTriggerFunctionName, customTrigger)
	})
}

func (suite *DeployFunctionTestSuite) TestHTTPTriggerServiceTypes() {

	// set platform default service type to nodePort
	suite.PlatformConfiguration.Kube.DefaultServiceType = v1.ServiceTypeNodePort

	// create function with service of type nodePort from platform default
	defaultNodePortFunctionName := "with-default-http-trigger-node-port"
	createNodePortTriggerFunctionOptions := suite.CompileCreateFunctionOptions(defaultNodePortFunctionName)
	createNodePortTriggerFunctionOptions.FunctionConfig.Spec.ServiceType = ""
	suite.DeployFunction(createNodePortTriggerFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		serviceInstance := &v1.Service{}
		suite.GetResourceAndUnmarshal("service", kubecommon.ServiceNameFromFunctionName(defaultNodePortFunctionName), serviceInstance)
		suite.Require().Equal(v1.ServiceTypeNodePort, serviceInstance.Spec.Type)
		return true
	})

	// set platform default service type to clusterIP - the rest of the test will use this default
	suite.PlatformConfiguration.Kube.DefaultServiceType = v1.ServiceTypeClusterIP

	// create function with service of type clusterIP from platform default
	defaultClusterIPFunctionName := "with-default-http-trigger-cluster-ip"
	createClusterIPTriggerFunctionOptions := suite.CompileCreateFunctionOptions(defaultClusterIPFunctionName)
	createClusterIPTriggerFunctionOptions.FunctionConfig.Spec.ServiceType = ""
	suite.DeployFunction(createClusterIPTriggerFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		serviceInstance := &v1.Service{}
		suite.GetResourceAndUnmarshal("service", kubecommon.ServiceNameFromFunctionName(defaultClusterIPFunctionName), serviceInstance)
		suite.Require().Equal(v1.ServiceTypeClusterIP, serviceInstance.Spec.Type)
		return true
	})

	// override default service type in function spec (backwards compatibility)
	customFunctionName := "custom-function"
	customFunctionOptions := suite.CompileCreateFunctionOptions(customFunctionName)
	customFunctionOptions.FunctionConfig.Spec.ServiceType = v1.ServiceTypeNodePort
	suite.DeployFunction(customFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		serviceInstance := &v1.Service{}
		suite.GetResourceAndUnmarshal("service", kubecommon.ServiceNameFromFunctionName(customFunctionName), serviceInstance)
		suite.Require().Equal(v1.ServiceTypeNodePort, serviceInstance.Spec.Type)
		return true
	})

	// override default service type in trigger spec
	customTriggerFunctionName := "with-default-http-trigger-cluster-ip"
	customTriggerFunctionOptions := suite.CompileCreateFunctionOptions(customTriggerFunctionName)
	customTrigger := functionconfig.Trigger{
		Kind:       "http",
		Name:       "custom-trigger",
		MaxWorkers: 1,
		Attributes: map[string]interface{}{
			"serviceType": v1.ServiceTypeNodePort,
		},
	}
	customTriggerFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		customTrigger.Name: customTrigger,
	}
	suite.DeployFunction(customTriggerFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		serviceInstance := &v1.Service{}
		suite.GetResourceAndUnmarshal("service", kubecommon.ServiceNameFromFunctionName(customTriggerFunctionName), serviceInstance)
		suite.Require().Equal(v1.ServiceTypeNodePort, serviceInstance.Spec.Type)
		return true
	})
}

type DeleteFunctionTestSuite struct {
	KubeTestSuite
}

func (suite *DeleteFunctionTestSuite) TestFailOnDeletingFunctionWithAPIGateways() {
	functionName := "func-to-delete"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		apiGatewayName := "func-apigw"
		createAPIGatewayOptions := suite.compileCreateAPIGatewayOptions(apiGatewayName, functionName)
		suite.deployAPIGateway(createAPIGatewayOptions, func(ingress *extensionsv1beta1.Ingress) {
			suite.Assert().Contains(ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName, functionName)

			// try to delete the function while it uses this api gateway
			err := suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{
				FunctionConfig: createFunctionOptions.FunctionConfig,
			})
			suite.Assert().Equal(platform.ErrFunctionIsUsedByAPIGateways, errors.RootCause(err))

		}, false)

		return true
	})
}

type DeployAPIGatewayTestSuite struct {
	KubeTestSuite
}

func (suite *DeployAPIGatewayTestSuite) TestDexAuthMode() {
	functionName := "some-function-name"
	apiGatewayName := "some-api-gateway-name"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	configOauth2ProxyURL := "config-oauth2-url"
	suite.PlatformConfiguration.IngressConfig = platformconfig.IngressConfig{
		Oauth2ProxyURL: configOauth2ProxyURL,
	}
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		createAPIGatewayOptions := suite.compileCreateAPIGatewayOptions(apiGatewayName, functionName)
		createAPIGatewayOptions.APIGatewayConfig.Spec.AuthenticationMode = ingress.AuthenticationModeOauth2
		suite.deployAPIGateway(createAPIGatewayOptions, func(ingress *extensionsv1beta1.Ingress) {
			suite.Assert().NotContains(ingress.Annotations, "nginx.ingress.kubernetes.io/auth-signin")
			suite.Assert().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/auth-url"], configOauth2ProxyURL)
		}, false)

		overrideOauth2ProxyURL := "override-oauth2-url"
		createAPIGatewayOptions = suite.compileCreateAPIGatewayOptions(apiGatewayName, functionName)
		createAPIGatewayOptions.APIGatewayConfig.Spec.AuthenticationMode = ingress.AuthenticationModeOauth2
		createAPIGatewayOptions.APIGatewayConfig.Spec.Authentication = &platform.APIGatewayAuthenticationSpec{
			DexAuth: &ingress.DexAuth{
				Oauth2ProxyURL:               overrideOauth2ProxyURL,
				RedirectUnauthorizedToSignIn: true,
			},
		}
		suite.deployAPIGateway(createAPIGatewayOptions, func(ingress *extensionsv1beta1.Ingress) {
			suite.Assert().Contains(ingress.Annotations, "nginx.ingress.kubernetes.io/auth-signin")
			suite.Assert().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/auth-signin"], overrideOauth2ProxyURL)
			suite.Assert().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/auth-url"], overrideOauth2ProxyURL)
		}, false)
		return true
	})
}

func TestPlatformTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(DeployFunctionTestSuite))
	suite.Run(t, new(DeployAPIGatewayTestSuite))
	suite.Run(t, new(DeleteFunctionTestSuite))
}
