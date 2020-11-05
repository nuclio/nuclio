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
	"github.com/nuclio/nuclio/pkg/platform/kube/monitoring"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeployTestSuite struct {
	KubeTestSuite
}

func (suite *DeployTestSuite) SetupSuite() {
	suite.KubeTestSuite.SetupSuite()

	// start controller in background
	go suite.Controller.Start() // nolint: errcheck
}

type DeployAPIGatewayTestSuite struct {
	DeployTestSuite
}

type DeployFunctionTestSuite struct {
	DeployTestSuite
}

type DeleteFunctionTestSuite struct {
	DeployTestSuite
}

func (suite *DeleteFunctionTestSuite) TestFailOnDeletingFunctionWithAPIGateways() {
	functionName := "func-to-delete"
	createFunctionOptions := suite.compileCreateFunctionOptions(functionName)
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

func (suite *DeployAPIGatewayTestSuite) TestDexAuthMode() {
	functionName := "some-function-name"
	apiGatewayName := "some-api-gateway-name"
	createFunctionOptions := suite.compileCreateFunctionOptions(functionName)
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

func (suite *DeployFunctionTestSuite) TestStaleResourceVersion() {
	var resourceVersion string

	createFunctionOptions := suite.compileCreateFunctionOptions("resource-schema")

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
		suite.DeployFunctionExpectError(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
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
	createFunctionOptions := suite.compileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.SecurityContext = &v1.PodSecurityContext{
		RunAsUser:  &runAsUserID,
		RunAsGroup: &runAsGroupID,
		FSGroup:    &fsGroup,
	}
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		deploymentInstance := &appsv1.Deployment{}
		suite.getResourceAndUnmarshal("deployment",
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
	createFunctionOptions := suite.compileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Meta.Labels = functionLabels
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		deploymentInstance := &appsv1.Deployment{}
		functionInstance := &nuclioio.NuclioFunction{}
		suite.getResourceAndUnmarshal("nucliofunction",
			functionName,
			functionInstance)
		suite.getResourceAndUnmarshal("deployment",
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
	createFunctionOptions := suite.compileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.MinReplicas = &two
	createFunctionOptions.FunctionConfig.Spec.MaxReplicas = &three
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		hpaInstance := &autoscalingv1.HorizontalPodAutoscaler{}
		suite.getResourceAndUnmarshal("hpa", kube.HPANameFromFunctionName(functionName), hpaInstance)
		suite.Require().Equal(two, int(*hpaInstance.Spec.MinReplicas))
		suite.Require().Equal(three, int(hpaInstance.Spec.MaxReplicas))
		return true
	})
}

func (suite *DeployFunctionTestSuite) TestDefaultHTTPTrigger() {
	defaultTriggerFunctionName := "with-default-http-trigger"
	createDefaultTriggerFunctionOptions := suite.compileCreateFunctionOptions(defaultTriggerFunctionName)
	suite.DeployFunction(createDefaultTriggerFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// ensure only 1 http trigger exists, always.
		suite.ensureTriggerAmount(defaultTriggerFunctionName, "http", 1)
		defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
		return suite.verifyCreatedTrigger(defaultTriggerFunctionName, defaultHTTPTrigger)
	})

	customTriggerFunctionName := "custom-http-trigger"
	createCustomTriggerFunctionOptions := suite.compileCreateFunctionOptions(customTriggerFunctionName)
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
	createNodePortTriggerFunctionOptions := suite.compileCreateFunctionOptions(defaultNodePortFunctionName)
	suite.DeployFunction(createNodePortTriggerFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		serviceInstance := &v1.Service{}
		suite.getResourceAndUnmarshal("service", kube.ServiceNameFromFunctionName(defaultNodePortFunctionName), serviceInstance)
		suite.Require().Equal(v1.ServiceTypeNodePort, serviceInstance.Spec.Type)
		return true
	})

	// set platform default service type to clusterIP - the rest of the test will use this default
	suite.PlatformConfiguration.Kube.DefaultServiceType = v1.ServiceTypeClusterIP

	// create function with service of type clusterIP from platform default
	defaultClusterIPFunctionName := "with-default-http-trigger-cluster-ip"
	createClusterIPTriggerFunctionOptions := suite.compileCreateFunctionOptions(defaultClusterIPFunctionName)
	suite.DeployFunction(createClusterIPTriggerFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		serviceInstance := &v1.Service{}
		suite.getResourceAndUnmarshal("service", kube.ServiceNameFromFunctionName(defaultClusterIPFunctionName), serviceInstance)
		suite.Require().Equal(v1.ServiceTypeClusterIP, serviceInstance.Spec.Type)
		return true
	})

	// override default service type in function spec (backwards compatibility)
	customFunctionName := "custom-function"
	customFunctionOptions := suite.compileCreateFunctionOptions(customFunctionName)
	customFunctionOptions.FunctionConfig.Spec.ServiceType = v1.ServiceTypeNodePort
	suite.DeployFunction(customFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		serviceInstance := &v1.Service{}
		suite.getResourceAndUnmarshal("service", kube.ServiceNameFromFunctionName(customFunctionName), serviceInstance)
		suite.Require().Equal(v1.ServiceTypeNodePort, serviceInstance.Spec.Type)
		return true
	})

	// override default service type in trigger spec
	customTriggerFunctionName := "with-default-http-trigger-cluster-ip"
	customTriggerFunctionOptions := suite.compileCreateFunctionOptions(customTriggerFunctionName)
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
		suite.getResourceAndUnmarshal("service", kube.ServiceNameFromFunctionName(customTriggerFunctionName), serviceInstance)
		suite.Require().Equal(v1.ServiceTypeNodePort, serviceInstance.Spec.Type)
		return true
	})
}

func (suite *DeployFunctionTestSuite) TestFunctionIsReadyAfterDeploymentFailure() {
	functionName := "function-recovery"
	createFunctionOptions := suite.compileCreateFunctionOptions(functionName)
	getFunctionOptions := &platform.GetFunctionsOptions{
		Name:      createFunctionOptions.FunctionConfig.Meta.Name,
		Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
	}

	// change blocking interval so test wont take so long
	oldMonitoringPostDeploymentMonitoringBlockingInterval := monitoring.PostDeploymentMonitoringBlockingInterval
	monitoring.PostDeploymentMonitoringBlockingInterval = 1 * time.Millisecond
	defer func() {

		// undo changes
		monitoring.PostDeploymentMonitoringBlockingInterval = oldMonitoringPostDeploymentMonitoringBlockingInterval
	}()

	suite.DeployFunction(createFunctionOptions, func(deployResults *platform.CreateFunctionResult) bool {

		// get the function
		function := suite.getFunction(getFunctionOptions)

		// ensure function is ready
		suite.Require().Equal(functionconfig.FunctionStateReady, function.GetStatus().State)

		// get function pod, first one is enough
		pod := suite.getFunctionPods(functionName)[0]

		// get node name on which function pod is running
		nodeName := pod.Spec.NodeName

		// mark the node as unschedulable, we want to evict the pod from there
		suite.Logger.InfoWith("Setting cluster node as unschedulable", "nodeName", nodeName)
		_, err := suite.KubeClientSet.CoreV1().Nodes().Update(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodeName,
				Namespace: suite.Namespace,
			},
			Spec: v1.NodeSpec{
				Unschedulable: true,
			},
		})
		suite.Require().NoError(err, "Failed to set nodes unschedulable")

		// no matter how this test ends up - ensure the node is schedulable again
		defer func() {
			_, err := suite.KubeClientSet.CoreV1().Nodes().Update(&v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeName,
					Namespace: suite.Namespace,
				},
				Spec: v1.NodeSpec{
					Unschedulable: false,
				},
			})
			suite.Require().NoError(err)
		}()

		// delete function pod
		zeroSeconds := int64(0)
		suite.Logger.InfoWith("Deleting function pod", "podName", pod.Name)
		err = suite.KubeClientSet.CoreV1().Pods(suite.Namespace).Delete(pod.Name,
			&metav1.DeleteOptions{
				GracePeriodSeconds: &zeroSeconds,
			})
		suite.Require().NoError(err, "Failed to delete function pod")

		// wait for controller to mark function in error due to pods are unschedulable
		err = common.RetryUntilSuccessful(2*suite.Controller.GetFunctionMonitorInterval(),
			1*time.Second,
			func() bool {
				function = suite.getFunction(getFunctionOptions)
				suite.Logger.InfoWith("Waiting for function state",
					"currentFunctionState", function.GetStatus().State,
					"expectedFunctionState", functionconfig.FunctionStateError)
				return function.GetStatus().State == functionconfig.FunctionStateError
			})
		suite.Require().NoError(err, "Failed to ensure function state is error")

		// mark k8s cluster nodes as schedulable
		suite.Logger.InfoWith("Setting cluster node as schedulable", "nodeName", nodeName)
		_, err = suite.KubeClientSet.CoreV1().Nodes().Update(&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nodeName,
				Namespace: suite.Namespace,
			},
			Spec: v1.NodeSpec{
				Unschedulable: false,
			},
		})
		suite.Require().NoError(err, "Failed to set nodes schedulable")

		// wait for function pods to run, meaning its deployment is available
		err = common.RetryUntilSuccessful(2*suite.Controller.GetFunctionMonitorInterval(),
			1*time.Second,
			func() bool {
				pod = suite.getFunctionPods(functionName)[0]
				suite.Logger.InfoWith("Waiting for function pod",
					"podName", pod.Name,
					"currentPodPhase", pod.Status.Phase,
					"expectedPodPhase", v1.PodRunning)
				return pod.Status.Phase == v1.PodRunning
			})
		suite.Require().NoError(err, "Failed to ensure function pod is running again")

		// wait for function state to become ready again
		err = common.RetryUntilSuccessful(2*suite.Controller.GetResyncInterval(),
			1*time.Second,
			func() bool {
				function = suite.getFunction(getFunctionOptions)
				suite.Logger.InfoWith("Waiting for function state",
					"currentFunctionState", function.GetStatus().State,
					"expectedFunctionState", functionconfig.FunctionStateReady)
				return function.GetStatus().State == functionconfig.FunctionStateReady
			})
		suite.Require().NoError(err, "Failed to ensure function is ready again")
		return true
	})
}

func (suite *DeployFunctionTestSuite) verifyCreatedTrigger(functionName string, trigger functionconfig.Trigger) bool {
	functionInstance := &nuclioio.NuclioFunction{}
	suite.getResourceAndUnmarshal("nucliofunction",
		functionName,
		functionInstance)

	// TODO: verify other parts of the trigger spec
	suite.Require().Equal(trigger.Name, functionInstance.Spec.Triggers[trigger.Name].Name)
	suite.Require().Equal(trigger.Kind, functionInstance.Spec.Triggers[trigger.Name].Kind)
	suite.Require().Equal(trigger.MaxWorkers, functionInstance.Spec.Triggers[trigger.Name].MaxWorkers)
	return true
}

func (suite *DeployFunctionTestSuite) ensureTriggerAmount(functionName, triggerKind string, amount int) {
	functionInstance := &nuclioio.NuclioFunction{}
	suite.getResourceAndUnmarshal("nucliofunction",
		functionName,
		functionInstance)

	functionHTTPTriggers := functionconfig.GetTriggersByKind(functionInstance.Spec.Triggers, triggerKind)
	suite.Require().Equal(amount, len(functionHTTPTriggers))
}

func (suite *DeployFunctionTestSuite) getFunction(getFunctionOptions *platform.GetFunctionsOptions) platform.Function {

	// get the function
	functions, err := suite.Platform.GetFunctions(getFunctionOptions)
	suite.Require().NoError(err)
	return functions[0]
}

func (suite *DeployFunctionTestSuite) getFunctionPods(functionName string) []v1.Pod {
	pods, err := suite.KubeClientSet.CoreV1().Pods(suite.Namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("nuclio.io/function-name=%s", functionName),
	})

	suite.Require().NoError(err, "Failed to list function pods")
	return pods.Items
}

func (suite *DeployTestSuite) compileCreateFunctionOptions(
	functionName string) *platform.CreateFunctionOptions {

	createFunctionOptions := suite.KubeTestSuite.compileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.Handler = "main:handler"
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:3.6"
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(`
def handler(context, event):
  return "hello world"
`))
	return createFunctionOptions
}

func (suite *DeployTestSuite) compileCreateAPIGatewayOptions(
	apiGatewayName string, functionName string) *platform.CreateAPIGatewayOptions {

	return &platform.CreateAPIGatewayOptions{
		APIGatewayConfig: platform.APIGatewayConfig{
			Meta: platform.APIGatewayMeta{
				Name:      apiGatewayName,
				Namespace: suite.Namespace,
			},
			Spec: platform.APIGatewaySpec{
				Host:               "some-host",
				AuthenticationMode: ingress.AuthenticationModeNone,
				Upstreams: []platform.APIGatewayUpstreamSpec{
					{
						Kind: platform.APIGatewayUpstreamKindNuclioFunction,
						Nucliofunction: &platform.NuclioFunctionAPIGatewaySpec{
							Name: functionName,
						},
					},
				},
			},
		},
	}
}

func TestPlatformTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(DeployFunctionTestSuite))
	suite.Run(t, new(DeployAPIGatewayTestSuite))
	suite.Run(t, new(DeleteFunctionTestSuite))
}
