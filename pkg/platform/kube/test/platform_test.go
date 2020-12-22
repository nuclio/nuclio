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
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/nuclio/errors"
	"github.com/rs/xid"
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

func (suite *DeployFunctionTestSuite) TestStaleResourceVersion() {
	var resourceVersion string

	createFunctionOptions := suite.CompileCreateFunctionOptions("resource-schema")

	afterFirstDeploy := func(deployResult *platform.CreateFunctionResult) bool {

		// get the function
		function := suite.GetFunction(&platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})

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
		function := suite.GetFunction(&platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})

		suite.Require().NotEqual(resourceVersion,
			function.GetConfig().Meta.ResourceVersion,
			"Resource version should be changed between deployments")

		// we expect a failure due to a stale resource version
		suite.DeployFunctionExpectError(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool { // nolint: errcheck
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
			kube.DeploymentNameFromFunctionName(functionName),
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
		podName := fmt.Sprintf("deployment/%s", kube.DeploymentNameFromFunctionName(functionName))
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
			kube.DeploymentNameFromFunctionName(functionName),
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
		suite.GetResourceAndUnmarshal("hpa", kube.HPANameFromFunctionName(functionName), hpaInstance)
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
		suite.GetResourceAndUnmarshal("service", kube.ServiceNameFromFunctionName(defaultNodePortFunctionName), serviceInstance)
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
		suite.GetResourceAndUnmarshal("service", kube.ServiceNameFromFunctionName(defaultClusterIPFunctionName), serviceInstance)
		suite.Require().Equal(v1.ServiceTypeClusterIP, serviceInstance.Spec.Type)
		return true
	})

	// override default service type in function spec (backwards compatibility)
	customFunctionName := "custom-function"
	customFunctionOptions := suite.CompileCreateFunctionOptions(customFunctionName)
	customFunctionOptions.FunctionConfig.Spec.ServiceType = v1.ServiceTypeNodePort
	suite.DeployFunction(customFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		serviceInstance := &v1.Service{}
		suite.GetResourceAndUnmarshal("service", kube.ServiceNameFromFunctionName(customFunctionName), serviceInstance)
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
		suite.GetResourceAndUnmarshal("service", kube.ServiceNameFromFunctionName(customTriggerFunctionName), serviceInstance)
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
		err := suite.deployAPIGateway(createAPIGatewayOptions, func(ingress *extensionsv1beta1.Ingress) {
			suite.Assert().Contains(ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName, functionName)

			// try to delete the function while it uses this api gateway
			err := suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{
				FunctionConfig: createFunctionOptions.FunctionConfig,
			})
			suite.Assert().Equal(platform.ErrFunctionIsUsedByAPIGateways, errors.RootCause(err))

		})
		suite.Require().NoError(err)

		return true
	})
}

func (suite *DeleteFunctionTestSuite) TestStaleResourceVersion() {
	var resourceVersion string

	createFunctionOptions := suite.CompileCreateFunctionOptions("delete-resource-schema")

	afterFirstDeploy := func(deployResult *platform.CreateFunctionResult) bool {

		// get the function
		function := suite.GetFunction(&platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})

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
		function := suite.GetFunction(&platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})

		suite.Require().NotEqual(resourceVersion,
			function.GetConfig().Meta.ResourceVersion,
			"Resource version should be changed between deployments")

		deployResult.UpdatedFunctionConfig.Meta.ResourceVersion = resourceVersion

		// expect a failure due to a stale resource version
		suite.Logger.Info("Deleting function")
		err := suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{
			FunctionConfig: deployResult.UpdatedFunctionConfig,
		})
		suite.Require().Error(err)

		deployResult.UpdatedFunctionConfig.Meta.ResourceVersion = function.GetConfig().Meta.ResourceVersion

		// succeeded delete function
		suite.Logger.Info("Deleting function")
		err = suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{
			FunctionConfig: deployResult.UpdatedFunctionConfig,
		})
		suite.Require().NoError(err)
		return true
	}

	suite.DeployFunctionAndRedeploy(createFunctionOptions, afterFirstDeploy, afterSecondDeploy)
}

type UpdateFunctionTestSuite struct {
	KubeTestSuite
}

func (suite *UpdateFunctionTestSuite) TestSanity() {
	createFunctionOptions := suite.CompileCreateFunctionOptions("update-sanity")
	createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{
		"something": "here",
	}
	createFunctionOptions.FunctionConfig.Meta.Annotations = map[string]string{
		"annotation-key": "annotation-value",
	}

	// create a disabled function
	zero := 0
	createFunctionOptions.FunctionConfig.Spec.Disable = true
	createFunctionOptions.FunctionConfig.Spec.Replicas = &zero
	_, err := suite.Platform.CreateFunction(createFunctionOptions)
	suite.Require().NoError(err, "Failed to create function")

	// delete leftovers
	defer func() {
		err = suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{
			FunctionConfig: createFunctionOptions.FunctionConfig,
		})
		suite.Require().NoError(err, "Failed to delete function")
	}()

	// change annotations
	createFunctionOptions.FunctionConfig.Meta.Annotations["annotation-key"] = "annotation-value-changed"
	createFunctionOptions.FunctionConfig.Meta.Annotations["added-annotation"] = "added"

	// update function
	err = suite.Platform.UpdateFunction(&platform.UpdateFunctionOptions{
		FunctionMeta: &createFunctionOptions.FunctionConfig.Meta,
		FunctionSpec: &createFunctionOptions.FunctionConfig.Spec,
	})
	suite.Require().NoError(err, "Failed to update function")

	// get function
	function := suite.GetFunction(&platform.GetFunctionsOptions{
		Name:      createFunctionOptions.FunctionConfig.Meta.Name,
		Namespace: suite.Namespace,
	})

	// ensure retrieved function equal to updated
	suite.Require().
		Empty(cmp.Diff(
			createFunctionOptions.FunctionConfig,
			*function.GetConfig(),
			cmp.Options{
				cmpopts.IgnoreFields(createFunctionOptions.FunctionConfig.Meta,
					"ResourceVersion"), // kubernetes opaque value
				cmpopts.IgnoreFields(createFunctionOptions.FunctionConfig.Spec,
					"Image", "ImageHash"), // auto generated during deploy

				// TODO: compare triggers as well
				// currently block due to serviceType being converted to string during get functions)
				cmpopts.IgnoreTypes(map[string]functionconfig.Trigger{}),
			},
		))
}

type DeployAPIGatewayTestSuite struct {
	KubeTestSuite
}

// test that api gateway cannot be created if one of its functions have ingresses
func (suite *DeployAPIGatewayTestSuite) TestAPIGatewayFunctionsHaveNoIngress() {
	functionName := "some-function-name"
	apiGatewayName := "some-api-gateway-name"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"some-http-trigger": {
			Kind: "http",
			Attributes: map[string]interface{}{
				"ingresses": map[string]interface{}{
					"1": map[string]interface{}{
						"host": "some-host",
						"paths": []string{
							"/some-path",
						},
					},
				},
			},
		},
	}

	// deploy a function with an ingress
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		createAPIGatewayOptions := suite.compileCreateAPIGatewayOptions(apiGatewayName, functionName)
		expectedErrorMessage := fmt.Sprintf("Api gateway upstream function: %s must not have an ingress", functionName)

		// try to create api gateway with this function as upstream and expect it to fail
		err := suite.deployAPIGateway(createAPIGatewayOptions, nil)
		suite.Require().Error(err)
		suite.Require().Equal(expectedErrorMessage, errors.RootCause(err).Error())

		return true
	})
}

// test that a function cannot expose ingresses if it is already being exposed by an api gateway
func (suite *DeployAPIGatewayTestSuite) TestUpdateFunctionWithIngressWhenHasAPIGateway() {
	functionName := "some-function-name"
	apiGatewayName := "some-api-gateway-name"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	// deploy a function
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// create an api-gateway with that function as upstream
		createAPIGatewayOptions := suite.compileCreateAPIGatewayOptions(apiGatewayName, functionName)
		err := suite.deployAPIGateway(createAPIGatewayOptions, func(*extensionsv1beta1.Ingress) {

			// update the function to have ingresses
			createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
				"some-http-trigger": {
					Kind: "http",
					Attributes: map[string]interface{}{
						"ingresses": map[string]interface{}{
							"1": map[string]interface{}{
								"host": "some-host",
								"paths": []string{
									"/some-path",
								},
							},
						},
					},
				},
			}

			// expect the function deployment to fail because it is already being exposed by an api gateway
			_, err := suite.DeployFunctionExpectError(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
				return true
			})
			suite.Require().Equal("Function can't expose ingresses while it is being exposed by an api gateway", errors.RootCause(err).Error())
		})
		suite.Require().NoError(err)

		return true
	})
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
		err := suite.deployAPIGateway(createAPIGatewayOptions, func(ingress *extensionsv1beta1.Ingress) {
			suite.Assert().NotContains(ingress.Annotations, "nginx.ingress.kubernetes.io/auth-signin")
			suite.Assert().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/auth-url"], configOauth2ProxyURL)
		})
		suite.Require().NoError(err)

		overrideOauth2ProxyURL := "override-oauth2-url"
		createAPIGatewayOptions = suite.compileCreateAPIGatewayOptions(apiGatewayName, functionName)
		createAPIGatewayOptions.APIGatewayConfig.Spec.AuthenticationMode = ingress.AuthenticationModeOauth2
		createAPIGatewayOptions.APIGatewayConfig.Spec.Authentication = &platform.APIGatewayAuthenticationSpec{
			DexAuth: &ingress.DexAuth{
				Oauth2ProxyURL:               overrideOauth2ProxyURL,
				RedirectUnauthorizedToSignIn: true,
			},
		}
		err = suite.deployAPIGateway(createAPIGatewayOptions, func(ingress *extensionsv1beta1.Ingress) {
			suite.Assert().Contains(ingress.Annotations, "nginx.ingress.kubernetes.io/auth-signin")
			suite.Assert().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/auth-signin"], overrideOauth2ProxyURL)
			suite.Assert().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/auth-url"], overrideOauth2ProxyURL)
		})
		suite.Require().NoError(err)

		return true
	})
}

func (suite *DeployAPIGatewayTestSuite) TestUpdate() {
	functionName := "function-name-" + xid.New().String()
	apiGatewayName := "apigw-name-" + xid.New().String()
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		createAPIGatewayOptions := suite.compileCreateAPIGatewayOptions(apiGatewayName, functionName)
		beforeUpdateHostValue := "before-update-host.com"
		createAPIGatewayOptions.APIGatewayConfig.Spec.Host = beforeUpdateHostValue

		// create
		err := suite.Platform.CreateAPIGateway(createAPIGatewayOptions)
		suite.Require().NoError(err)

		// delete leftovers
		defer suite.Platform.DeleteAPIGateway(&platform.DeleteAPIGatewayOptions{ // nolint: errcheck
			Meta: createAPIGatewayOptions.APIGatewayConfig.Meta,
		})

		suite.WaitForAPIGatewayState(&platform.GetAPIGatewaysOptions{
			Name:      createAPIGatewayOptions.APIGatewayConfig.Meta.Name,
			Namespace: createAPIGatewayOptions.APIGatewayConfig.Meta.Namespace,
		}, platform.APIGatewayStateReady, 10*time.Second)

		ingressInstance := suite.GetAPIGatewayIngress(createAPIGatewayOptions.APIGatewayConfig.Meta.Name, false)
		suite.Require().Equal(beforeUpdateHostValue, ingressInstance.Spec.Rules[0].Host)

		// change host, update
		afterUpdateHostValue := "after-update-host.com"
		annotations := map[string]string{
			"annotation-key": "annotation-value",
		}
		createAPIGatewayOptions.APIGatewayConfig.Spec.Host = afterUpdateHostValue
		createAPIGatewayOptions.APIGatewayConfig.Meta.Annotations = annotations
		err = suite.Platform.UpdateAPIGateway(&platform.UpdateAPIGatewayOptions{
			APIGatewayConfig: createAPIGatewayOptions.APIGatewayConfig,
		})
		suite.Require().NoError(err)

		getAPIGatewayOptions := &platform.GetAPIGatewaysOptions{
			Name:      createAPIGatewayOptions.APIGatewayConfig.Meta.Name,
			Namespace: createAPIGatewayOptions.APIGatewayConfig.Meta.Namespace,
		}
		suite.WaitForAPIGatewayState(getAPIGatewayOptions, platform.APIGatewayStateReady, 10*time.Second)

		ingressInstance = suite.GetAPIGatewayIngress(createAPIGatewayOptions.APIGatewayConfig.Meta.Name, false)
		suite.Require().Equal(afterUpdateHostValue, ingressInstance.Spec.Rules[0].Host)

		apiGateway := suite.GetAPIGateway(getAPIGatewayOptions)
		suite.Require().Equal(annotations, apiGateway.GetConfig().Meta.Annotations)
		return true
	})
}

type ProjectTestSuite struct {
	KubeTestSuite
}

func (suite *ProjectTestSuite) TestCreate() {
	projectConfig := platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:      "test-project",
			Namespace: suite.Namespace,
			Labels: map[string]string{
				"label-key": "label-value",
			},
			Annotations: map[string]string{
				"annotation-key": "annotation-value",
			},
		},
		Spec: platform.ProjectSpec{
			Description: "some description",
		},
	}

	// create project
	err := suite.Platform.CreateProject(&platform.CreateProjectOptions{
		ProjectConfig: &projectConfig,
	})
	suite.Require().NoError(err, "Failed to create project")
	defer func() {
		err = suite.Platform.DeleteProject(&platform.DeleteProjectOptions{
			Meta:     projectConfig.Meta,
			Strategy: platform.DeleteProjectStrategyRestrict,
		})
		suite.Require().NoError(err, "Failed to delete project")
	}()

	// get created project
	projects, err := suite.Platform.GetProjects(&platform.GetProjectsOptions{
		Meta: projectConfig.Meta,
	})
	suite.Require().NoError(err, "Failed to get projects")
	suite.Require().Equal(len(projects), 1)

	// requested and created project are equal
	createdProject := projects[0]
	suite.Require().Equal(projectConfig, *createdProject.GetConfig())
}

func (suite *ProjectTestSuite) TestUpdate() {
	projectConfig := platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:      "test-project",
			Namespace: suite.Namespace,
			Labels: map[string]string{
				"something": "here",
			},
			Annotations: map[string]string{
				"annotation-key": "annotation-value",
			},
		},
		Spec: platform.ProjectSpec{
			Description: "Simple description",
		},
	}

	// create project
	err := suite.Platform.CreateProject(&platform.CreateProjectOptions{
		ProjectConfig: &projectConfig,
	})
	suite.Require().NoError(err, "Failed to create project")

	// delete leftover
	defer func() {
		err = suite.Platform.DeleteProject(&platform.DeleteProjectOptions{
			Meta:     projectConfig.Meta,
			Strategy: platform.DeleteProjectStrategyRestrict,
		})
		suite.Require().NoError(err, "Failed to delete project")
	}()

	// change project annotations
	projectConfig.Meta.Annotations["annotation-key"] = "annotation-value-changed"
	projectConfig.Meta.Annotations["added-annotation"] = "added-annotation-value"

	// change project labels
	projectConfig.Meta.Labels["label-key"] = "label-value-changed"
	projectConfig.Meta.Labels["added-label"] = "added-label-value"

	// update project
	err = suite.Platform.UpdateProject(&platform.UpdateProjectOptions{
		ProjectConfig: projectConfig,
	})
	suite.Require().NoError(err, "Failed to update project")

	// get updated project
	updatedProject := suite.GetProject(&platform.GetProjectsOptions{
		Meta: projectConfig.Meta,
	})
	suite.Require().Empty(cmp.Diff(projectConfig, *updatedProject.GetConfig()))
}

func (suite *ProjectTestSuite) TestDelete() {
	projectConfig := platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:      "test-project",
			Namespace: suite.Namespace,
			Labels: map[string]string{
				"something": "here",
			},
			Annotations: map[string]string{
				"annotation-key": "annotation-value",
			},
		},
		Spec: platform.ProjectSpec{
			Description: "Simple description",
		},
	}

	// create project
	err := suite.Platform.CreateProject(&platform.CreateProjectOptions{
		ProjectConfig: &projectConfig,
	})
	suite.Require().NoError(err, "Failed to create project")

	// delete project
	err = suite.Platform.DeleteProject(&platform.DeleteProjectOptions{
		Meta:     projectConfig.Meta,
		Strategy: platform.DeleteProjectStrategyRestrict,
	})
	suite.Require().NoError(err, "Failed to delete project")

	// ensure project does not exists
	projects, err := suite.Platform.GetProjects(&platform.GetProjectsOptions{
		Meta: projectConfig.Meta,
	})
	suite.Require().NoError(err, "Failed to get projects")
	suite.Require().Equal(len(projects), 0)
}

func (suite *ProjectTestSuite) TestDeleteCascade() {

	// create project
	projectToDeleteConfig := platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:      "project-to-delete",
			Namespace: suite.Namespace,
		},
	}
	err := suite.Platform.CreateProject(&platform.CreateProjectOptions{
		ProjectConfig: &projectToDeleteConfig,
	})
	suite.Require().NoError(err, "Failed to create project")

	// create 2 functions (deleted along with `projectToDeleteConfig`)

	// create function A
	functionToDeleteA := suite.CompileCreateFunctionOptions("func-to-delete-a")
	functionToDeleteA.FunctionConfig.Meta.Annotations = map[string]string{
		functionconfig.FunctionAnnotationSkipBuild:  "true",
		functionconfig.FunctionAnnotationSkipDeploy: "true",
	}
	functionToDeleteA.FunctionConfig.Meta.Labels["nuclio.io/project-name"] = projectToDeleteConfig.Meta.Name
	suite.PopulateDeployOptions(functionToDeleteA)
	_, err = suite.Platform.CreateFunction(functionToDeleteA)
	suite.Require().NoError(err)
	defer suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{ // nolint: errcheck
		FunctionConfig: functionToDeleteA.FunctionConfig,
	})
	suite.WaitForFunctionState(&platform.GetFunctionsOptions{
		Name:      functionToDeleteA.FunctionConfig.Meta.Name,
		Namespace: suite.Namespace,
	}, functionconfig.FunctionStateImported, time.Minute)

	// create function B
	functionToDeleteB := suite.CompileCreateFunctionOptions("func-to-delete-b")
	functionToDeleteB.FunctionConfig.Meta.Annotations = map[string]string{
		functionconfig.FunctionAnnotationSkipBuild:  "true",
		functionconfig.FunctionAnnotationSkipDeploy: "true",
	}
	functionToDeleteB.FunctionConfig.Meta.Labels["nuclio.io/project-name"] = projectToDeleteConfig.Meta.Name
	suite.PopulateDeployOptions(functionToDeleteB)
	_, err = suite.Platform.CreateFunction(functionToDeleteB)
	suite.Require().NoError(err)
	defer suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{ // nolint: errcheck
		FunctionConfig: functionToDeleteB.FunctionConfig,
	})
	suite.WaitForFunctionState(&platform.GetFunctionsOptions{
		Name:      functionToDeleteB.FunctionConfig.Meta.Name,
		Namespace: suite.Namespace,
	}, functionconfig.FunctionStateImported, time.Minute)

	// create api gateway for function A (deleted along with `projectToDeleteConfig`)
	createAPIGatewayOptions := suite.compileCreateAPIGatewayOptions("apigw-to-delete",
		functionToDeleteA.FunctionConfig.Meta.Name)
	createAPIGatewayOptions.APIGatewayConfig.Meta.Labels["nuclio.io/project-name"] = projectToDeleteConfig.Meta.Name
	err = suite.Platform.CreateAPIGateway(createAPIGatewayOptions)
	suite.Require().NoError(err)
	defer suite.Platform.DeleteAPIGateway(&platform.DeleteAPIGatewayOptions{ // nolint: errcheck
		Meta: createAPIGatewayOptions.APIGatewayConfig.Meta,
	})

	suite.WaitForAPIGatewayState(&platform.GetAPIGatewaysOptions{
		Name:      createAPIGatewayOptions.APIGatewayConfig.Meta.Name,
		Namespace: createAPIGatewayOptions.APIGatewayConfig.Meta.Namespace,
	}, platform.APIGatewayStateReady, 10*time.Second)

	// create 2 function events for function B (deleted along with `projectToDeleteConfig`)
	functionEventA := suite.CompileCreateFunctionEventOptions("function-event-a",
		functionToDeleteB.FunctionConfig.Meta.Name)
	err = suite.Platform.CreateFunctionEvent(functionEventA)
	suite.Require().NoError(err)
	defer suite.Platform.DeleteFunctionEvent(&platform.DeleteFunctionEventOptions{ // nolint: errcheck
		Meta: platform.FunctionEventMeta{
			Name:      functionEventA.FunctionEventConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
	})

	functionEventB := suite.CompileCreateFunctionEventOptions("function-event-b",
		functionToDeleteB.FunctionConfig.Meta.Name)
	err = suite.Platform.CreateFunctionEvent(functionEventB)
	suite.Require().NoError(err)
	defer suite.Platform.DeleteFunctionEvent(&platform.DeleteFunctionEventOptions{ // nolint: errcheck
		Meta: platform.FunctionEventMeta{
			Name:      functionEventB.FunctionEventConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
	})

	// try restrict - expect it to fail (project has sub resources)
	err = suite.Platform.DeleteProject(&platform.DeleteProjectOptions{
		Meta: platform.ProjectMeta{
			Name:      projectToDeleteConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
		Strategy: platform.DeleteProjectStrategyRestrict,
	})
	suite.Require().Error(err)

	// try cascade - expect it succeed
	err = suite.Platform.DeleteProject(&platform.DeleteProjectOptions{
		Meta: platform.ProjectMeta{
			Name:      projectToDeleteConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
		Strategy: platform.DeleteProjectStrategyCascade,
	})
	suite.Require().NoError(err)

	// assertion - project should be deleted
	projects, err := suite.Platform.GetProjects(&platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      projectToDeleteConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
	})
	suite.Require().NoError(err)
	suite.Require().Len(projects, 0)

	suite.Logger.InfoWith("Ensuring resources were removed (deletion is being executed in background")

	// ensure api gateway deleted
	err = common.RetryUntilSuccessful(time.Minute, 3*time.Second, func() bool {
		apiGateways, err := suite.Platform.GetAPIGateways(&platform.GetAPIGatewaysOptions{
			Name:      createAPIGatewayOptions.APIGatewayConfig.Meta.Name,
			Namespace: suite.Namespace,
		})
		suite.Require().NoError(err)
		exists := len(apiGateways) == 0
		if exists {
			suite.Logger.DebugWith("Waiting for api gateway to be deleted",
				"name", createAPIGatewayOptions.APIGatewayConfig.Meta.Name)
		}
		return len(apiGateways) == 0
	})
	suite.Require().NoError(err)

	// ensure functions were deleted successfully
	for _, functionName := range []string{
		functionToDeleteA.FunctionConfig.Meta.Name,
		functionToDeleteB.FunctionConfig.Meta.Name,
	} {
		err = common.RetryUntilSuccessful(time.Minute, 3*time.Second, func() bool {
			functions, err := suite.Platform.GetFunctions(&platform.GetFunctionsOptions{
				Name:      functionName,
				Namespace: suite.Namespace,
			})
			suite.Require().NoError(err)
			exists := len(functions) == 0
			if exists {
				suite.Logger.DebugWith("Waiting for function to be deleted",
					"name", createAPIGatewayOptions.APIGatewayConfig.Meta.Name)
			}
			return len(functions) == 0
		})
		suite.Require().NoError(err)
	}

	// ensure function events were deleted successfully
	for _, functionEventName := range []string{
		functionEventA.FunctionEventConfig.Meta.Name,
		functionEventB.FunctionEventConfig.Meta.Name,
	} {
		err = common.RetryUntilSuccessful(time.Minute, 3*time.Second, func() bool {
			functionEvents, err := suite.Platform.GetFunctionEvents(&platform.GetFunctionEventsOptions{
				Meta: platform.FunctionEventMeta{
					Name:      functionEventName,
					Namespace: suite.Namespace,
				},
			})
			suite.Require().NoError(err)
			exists := len(functionEvents) == 0
			if exists {
				suite.Logger.DebugWith("Waiting for function event to be deleted",
					"name", createAPIGatewayOptions.APIGatewayConfig.Meta.Name)
			}
			return len(functionEvents) == 0
		})
		suite.Require().NoError(err)
	}
}

func TestPlatformTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(DeployFunctionTestSuite))
	suite.Run(t, new(UpdateFunctionTestSuite))
	suite.Run(t, new(DeleteFunctionTestSuite))
	suite.Run(t, new(DeployAPIGatewayTestSuite))
	suite.Run(t, new(ProjectTestSuite))
}
