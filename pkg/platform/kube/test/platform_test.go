//go:build test_integration && test_kube

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

package test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
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
	"github.com/nuclio/nuclio/pkg/processor/trigger/cron"

	"github.com/gobuffalo/flect"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/nuclio/errors"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	autosv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type DeployFunctionTestSuite struct {
	KubeTestSuite
}

func (suite *DeployFunctionTestSuite) TestDeployCronTriggerK8sWithJSONEventBody() {

	// create an event-recorder function
	functionPath := path.Join(suite.GetTestFunctionsDir(), "common", "event-recorder", "python", "event_recorder.py")
	functionName := fmt.Sprintf("event-recorder-%s", xid.New().String())
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	// get function source code
	functionSourceCode, err := os.ReadFile(functionPath)
	suite.Require().NoError(err)

	createFunctionOptions.FunctionConfig.Spec.Runtime = "python"
	createFunctionOptions.FunctionConfig.Spec.Handler = "event_recorder:handler"
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString(functionSourceCode)

	// compile cron trigger
	cronTriggerEvent := cron.Event{
		Body: `{"key_a":true,"key_b":["value_1","value_2"]}`,
		Headers: map[string]interface{}{
			"Extra-Header-1": "value1",
			"Extra-Header-2": "value2",
		},
	}
	cronTrigger := functionconfig.Trigger{
		Name: "cronTrigger",
		Kind: "cron",
		Attributes: map[string]interface{}{
			"interval": "5s",
			"event":    cronTriggerEvent,
		},
	}
	createFunctionOptions.FunctionConfig.Spec.ServiceType = v1.ServiceTypeClusterIP
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		cronTrigger.Name: cronTrigger,
	}

	// deploy function
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// try to get function recorded events for 60 seconds
		// we expect the function to record and return all cron triggers events
		var events []cron.Event
		suite.TryGetAndUnmarshalFunctionRecordedEvents(suite.Ctx, functionName, 60*time.Second, &events)
		firstEvent := events[0]

		// ensure recorded event
		suite.Require().Empty(cmp.Diff(cronTriggerEvent.Body, firstEvent.Body))
		for headerName := range cronTriggerEvent.Headers {
			suite.Require().Empty(cmp.Diff(cronTriggerEvent.Headers[headerName], firstEvent.Headers[headerName]))
		}
		return true
	})
}

// Test that we get the expected brief error message on function deployment failure
func (suite *DeployFunctionTestSuite) TestDeployFailureBriefErrorMessage() {
	platformConfigConfigmap := suite.createPlatformConfigmapWithJSONLogger()

	// delete the platform config map when this test is over
	defer suite.KubeClientSet.
		CoreV1().
		ConfigMaps(suite.Namespace).
		Delete(suite.Ctx, platformConfigConfigmap.Name, metav1.DeleteOptions{}) // nolint: errcheck

	for _, testCase := range []struct {
		Name                       string
		CreateFunctionOptions      *platform.CreateFunctionOptions
		ExpectedBriefErrorsMessage string
	}{
		{
			Name: "GoBadHandler",
			CreateFunctionOptions: func() *platform.CreateFunctionOptions {
				createFunctionOptions := suite.CompileCreateFunctionOptions("fail-func-go-bad-handler")
				createFunctionOptions.FunctionConfig.Spec.Runtime = "golang"
				createFunctionOptions.FunctionConfig.Spec.Handler = "main:ExpectedHandler"
				functionSourceCode := `package main
import (
  "github.com/nuclio/nuclio-sdk-go"
)
func NotExpectedHandler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
  return nil, nil
}`
				createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(functionSourceCode))
				return createFunctionOptions
			}(),
			ExpectedBriefErrorsMessage: `
Error - plugin: symbol ExpectedHandler not found in plugin github.com/nuclio/nuclio
    .../pkg/processor/runtime/golang/pluginloader.go:58
`,
		},
		{
			Name: "PythonBadHandler",
			CreateFunctionOptions: func() *platform.CreateFunctionOptions {
				createFunctionOptions := suite.CompileCreateFunctionOptions("fail-func-python-bad-handler")
				createFunctionOptions.FunctionConfig.Spec.Runtime = "python"
				createFunctionOptions.FunctionConfig.Spec.Handler = "main:expected_handler"
				functionSourceCode := `def not_expected_handler(context, event):
   return ""
`
				createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(functionSourceCode))
				return createFunctionOptions
			}(),

			// TODO: remove deprecation notice once python 3.6 is made obsolete
			ExpectedBriefErrorsMessage: `Python 3.6 runtime is deprecated and will soon not be supported. Please migrate your code and use Python 3.7 runtime or higher
Handler not found [handler="main:expected_handler" || worker_id="0"]
Caught unhandled exception while initializing [err="module 'main' has no attribute 'expected_handler'" || traceback="Traceback (most recent call last):
  File "/opt/nuclio/_nuclio_wrapper.py", line 409, in run_wrapper
    args.trigger_name)
  File "/opt/nuclio/_nuclio_wrapper.py", line 71, in __init__
    self._entrypoint = self._load_entrypoint_from_handler(handler)
  File "/opt/nuclio/_nuclio_wrapper.py", line 200, in _load_entrypoint_from_handler
    entrypoint_address = getattr(module, entrypoint)
AttributeError: module 'main' has no attribute 'expected_handler'
" || worker_id="0"]`,
		},
		{
			Name: "InsufficientGPU",
			CreateFunctionOptions: func() *platform.CreateFunctionOptions {
				createFunctionOptions := suite.CompileCreateFunctionOptions("fail-func-insufficient-gpu")
				createFunctionOptions.FunctionConfig.Spec.Runtime = "python"
				createFunctionOptions.FunctionConfig.Spec.Handler = "main:handler"
				functionSourceCode := `def handler(context, event): return ""`
				createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(functionSourceCode))
				createFunctionOptions.FunctionConfig.Spec.Resources.Limits = map[v1.ResourceName]resource.Quantity{
					functionconfig.NvidiaGPUResourceName: resource.MustParse("99"),
				}
				return createFunctionOptions
			}(),

			// on k8s <= 1.23, the error message is:
			// 	0/1 nodes are available: 1 Insufficient nvidia.com/gpu.
			// on k8s >= 1.24, the error message is:
			// 0/1 nodes are available: 1 Insufficient nvidia.com/gpu. preemption: 0/1 nodes are available:
			// 1 No preemption victims found for incoming pod.
			ExpectedBriefErrorsMessage: "0/1 nodes are available: 1 Insufficient nvidia.com/gpu. " +
				"preemption: 0/1 nodes are available: 1 No preemption victims found for incoming pod.\n",
		},
	} {
		suite.Run(testCase.Name, func() {
			_, err := suite.DeployFunctionExpectError(testCase.CreateFunctionOptions,
				func(deployResult *platform.CreateFunctionResult) bool {

					// get the function
					function := suite.GetFunction(&platform.GetFunctionsOptions{
						Name:      testCase.CreateFunctionOptions.FunctionConfig.Meta.Name,
						Namespace: testCase.CreateFunctionOptions.FunctionConfig.Meta.Namespace,
					})

					// validate the brief error message in function status is at least 90% close to the expected brief error message
					// keep it flexible for close enough messages in case small changes occur (e.g. line numbers on stack trace)
					briefErrorMessageDiff := common.CompareTwoStrings(testCase.ExpectedBriefErrorsMessage, function.GetStatus().Message)
					suite.Require().GreaterOrEqual(briefErrorMessageDiff, float32(0.80))

					return true
				})
			suite.Require().Error(err)

			err = suite.Platform.DeleteFunction(suite.Ctx, &platform.DeleteFunctionOptions{
				FunctionConfig: testCase.CreateFunctionOptions.FunctionConfig,
			})
			suite.Require().NoError(err)
		})
	}
}

func (suite *DeployFunctionTestSuite) TestVolumeOnceMountTwice() {
	functionName := "volume-once-mount-twice"
	volumeName := "some-volume"
	configMapName := "some-configmap"
	configMapData := xid.New().String()
	mountPaths := []string{"/etc/path/1", "/etc/path/2"}
	configMap, err := suite.KubeClientSet.
		CoreV1().
		ConfigMaps(suite.Namespace).
		Create(suite.Ctx,
			&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: suite.Namespace,
				},
				Data: map[string]string{
					"key": configMapData,
				},
			},
			metav1.CreateOptions{})
	suite.Require().NoError(err)

	// delete leftovers
	defer suite.KubeClientSet.
		CoreV1().
		ConfigMaps(suite.Namespace).
		Delete(suite.Ctx, configMap.Name, metav1.DeleteOptions{}) // nolint: errcheck

	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.Volumes = []functionconfig.Volume{}
	for _, mountPath := range mountPaths {
		createFunctionOptions.FunctionConfig.Spec.Volumes = append(createFunctionOptions.FunctionConfig.Spec.Volumes, functionconfig.Volume{
			Volume: v1.Volume{
				Name: volumeName,
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: configMap.GetName(),
						},
					},
				},
			},
			VolumeMount: v1.VolumeMount{
				Name:      volumeName,
				MountPath: mountPath,
			},
		})
	}

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		podName := fmt.Sprintf("deployment/%s", kube.DeploymentNameFromFunctionName(functionName))
		for _, mountPath := range mountPaths {
			results, err := suite.executeKubectl([]string{
				"exec",
				podName,
				"--",
				fmt.Sprintf("cat %s/key", mountPath),
			}, nil)
			suite.Require().NoError(err)
			suite.Require().Equal(configMapData, results.Output)
		}
		return true
	})
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
		suite.DeployFunctionExpectError(createFunctionOptions, // nolint: errcheck
			func(deployResult *platform.CreateFunctionResult) bool {
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

func (suite *DeployFunctionTestSuite) TestAssigningFunctionPodToNodes() {

	// TODO: currently is not working on minikube
	suite.T().Skip("Run manually")
	existingNodes := suite.GetNodes()
	suite.Require().NotEmpty(existingNodes, "Must have at least one node available")

	testNodeName := existingNodes[0].GetName()
	testLabelKey := "test-nuclio.io"

	labelPatch := fmt.Sprintf(`[{"op":"add","path":"/metadata/labels/%s","value":"%s"}]`, testLabelKey, "true")
	_, err := suite.KubeClientSet.CoreV1().Nodes().Patch(suite.Ctx, testNodeName, types.JSONPatchType, []byte(labelPatch), metav1.PatchOptions{})
	suite.Require().NoError(err, "Failed to patch node labels")

	// undo changes
	defer func() {
		suite.Logger.DebugWith("Rolling back node labels change")
		labelPatch = fmt.Sprintf(`[{"op":"remove","path":"/metadata/labels/%s"}]`, testLabelKey)
		_, err := suite.KubeClientSet.CoreV1().Nodes().Patch(suite.Ctx, testNodeName, types.JSONPatchType, []byte(labelPatch), metav1.PatchOptions{})
		suite.Require().NoError(err, "Failed to patch node labels")
	}()

	for _, testCase := range []struct {
		name string

		// function spec
		nodeName        string
		nodeSelector    map[string]string
		nodeAffinity    *v1.Affinity
		expectedFailure bool
	}{
		{
			name:     "AssignByNodeName",
			nodeName: testNodeName,
		},
		{
			name:            "UnscheduledNoSuchNodeName",
			nodeName:        "nuclio-do-not-should-not-exists",
			expectedFailure: true,
		},
		{
			name: "AssignByNodeSelector",
			nodeSelector: map[string]string{
				testLabelKey: "true",
			},
		},
		{
			name: "UnscheduledNoSuchLabel",
			nodeSelector: map[string]string{
				testLabelKey: "false",
			},
			expectedFailure: true,
		},
	} {
		suite.Run(testCase.name, func() {
			functionName := flect.Dasherize(testCase.name)
			createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
			createFunctionOptions.FunctionConfig.Spec.NodeName = testCase.nodeName
			createFunctionOptions.FunctionConfig.Spec.Affinity = testCase.nodeAffinity
			createFunctionOptions.FunctionConfig.Spec.NodeSelector = testCase.nodeSelector
			if testCase.expectedFailure {

				// dont wait for too long
				createFunctionOptions.FunctionConfig.Spec.ReadinessTimeoutSeconds = 15
				_, err := suite.DeployFunctionExpectError(createFunctionOptions,
					func(deployResult *platform.CreateFunctionResult) bool {
						return true
					})
				suite.Require().Error(err)

				if testCase.nodeSelector != nil {
					pods := suite.GetFunctionPods(functionName)
					podEvents, err := suite.KubeClientSet.CoreV1().Events(suite.Namespace).List(suite.Ctx, metav1.ListOptions{
						FieldSelector: fmt.Sprintf("involvedObject.name=%s", pods[0].GetName()),
					})
					suite.Require().NoError(err)
					suite.Require().NotNil(podEvents)
					suite.Require().Equal("FailedScheduling", podEvents.Items[0].Reason)
				}
			} else {
				suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
					pods := suite.GetFunctionPods(functionName)
					if testCase.nodeName != "" {
						suite.Require().Equal(testCase.nodeName, pods[0].Spec.NodeName)
					}
					if testCase.nodeSelector != nil {
						suite.Require().Equal(testCase.nodeSelector, pods[0].Spec.NodeSelector)
					}
					if testCase.nodeAffinity != nil {
						suite.Require().Equal(testCase.nodeAffinity, pods[0].Spec.Affinity)
					}
					return true
				})
			}
		})
	}
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
		hpaInstance := &autosv2.HorizontalPodAutoscaler{}
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

	// create a function with a nil service type
	nilServiceTypeFunctionName := "with-nil-service-type"
	nilServiceTypeFunctionOptions := suite.CompileCreateFunctionOptions(nilServiceTypeFunctionName)
	triggerAttributesJSON := `{ "serviceType": null }`
	triggerAttributes := map[string]interface{}{}
	err := json.Unmarshal([]byte(triggerAttributesJSON), &triggerAttributes)
	suite.Require().NoError(err)
	nilServiceTypeTrigger := functionconfig.Trigger{
		Kind:       "http",
		Name:       "nil-service-type-trigger",
		MaxWorkers: 1,
		Attributes: triggerAttributes,
	}
	nilServiceTypeFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		nilServiceTypeTrigger.Name: nilServiceTypeTrigger,
	}
	suite.DeployFunction(nilServiceTypeFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		serviceInstance := &v1.Service{}
		suite.GetResourceAndUnmarshal("service", kube.ServiceNameFromFunctionName(nilServiceTypeFunctionName), serviceInstance)
		suite.Require().Equal(v1.ServiceTypeNodePort, serviceInstance.Spec.Type)
		return true
	})

}

func (suite *DeployFunctionTestSuite) createPlatformConfigmapWithJSONLogger() *v1.ConfigMap {

	// create a platform config configmap with a json logger sink (this is how it is on production)
	platformConfigConfigmap, err := suite.KubeClientSet.
		CoreV1().
		ConfigMaps(suite.Namespace).
		Create(suite.Ctx,
			&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nuclio-platform-config",
					Namespace: suite.Namespace,
				},
				Data: map[string]string{
					"platform.yaml": `logger:
  functions:
  - level: debug
    sink: myStdoutLoggerSink
  sinks:
    myStdoutLoggerSink:
      attributes:
        encoding: json
        timeFieldEncoding: iso8601
        timeFieldName: time
        varGroupName: more
      kind: stdout
  system:
  - level: debug
    sink: myStdoutLoggerSink`,
				},
			},
			metav1.CreateOptions{})
	suite.Require().NoError(err)

	return platformConfigConfigmap
}

func (suite *DeployFunctionTestSuite) TestCreateFunctionWithIngress() {
	functionName := "func-with-ingress"
	ingressHost := "something.com"
	pathType := networkingv1.PathTypeImplementationSpecific
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"customTrigger": {
			Kind:       "http",
			Name:       "customTrigger",
			MaxWorkers: 3,
			Attributes: map[string]interface{}{
				"ingresses": map[string]interface{}{
					"someKey": map[string]interface{}{
						"paths":    []string{"/"},
						"pathType": &pathType,
						"host":     ingressHost,
					},
				},
			},
		},
	}

	suite.DeployFunctionAndRedeploy(createFunctionOptions,
		func(deployResult *platform.CreateFunctionResult) bool {

			// wait for function to become ready
			// that ensure us all of its resources (ingresses) are created correctly
			suite.WaitForFunctionState(&platform.GetFunctionsOptions{
				Name:      functionName,
				Namespace: suite.Namespace,
			},
				functionconfig.FunctionStateReady,
				time.Minute,
			)

			functionIngress := suite.GetFunctionIngress(functionName)
			suite.Require().Equal(ingressHost, functionIngress.Spec.Rules[0].Host)
			return true

		}, func(deployResult *platform.CreateFunctionResult) bool {

			// sanity check, redeploy does break on certain ingress / apigateway ingress validations
			return true
		})
}

func (suite *DeployFunctionTestSuite) TestCreateFunctionWithTemplatedIngress() {
	functionName := "func-with-ingress"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"customTrigger": {
			Kind:       "http",
			Name:       "customTrigger",
			MaxWorkers: 3,
			Attributes: map[string]interface{}{
				"ingresses": map[string]interface{}{
					"someKey": map[string]interface{}{
						"paths":        []string{"/"},
						"hostTemplate": "{{ .ResourceName }}.{{ .Namespace }}.nuclio.com",
					},
				},
			},
		},
	}

	suite.DeployFunctionAndRedeploy(createFunctionOptions,
		func(deployResult *platform.CreateFunctionResult) bool {

			// wait for function to become ready
			// that ensure us all of its resources (ingresses) are created correctly
			suite.WaitForFunctionState(&platform.GetFunctionsOptions{
				Name:      functionName,
				Namespace: suite.Namespace,
			}, functionconfig.FunctionStateReady, time.Minute)

			expectedIngressHost := fmt.Sprintf("%s.%s.nuclio.com",
				functionName,
				createFunctionOptions.FunctionConfig.Meta.Namespace)
			functionIngress := suite.GetFunctionIngress(functionName)
			suite.Require().Equal(expectedIngressHost, functionIngress.Spec.Rules[0].Host)
			return true

		}, func(deployResult *platform.CreateFunctionResult) bool {

			// sanity check, redeploy does break on certain ingress / apigateway ingress validations
			return true
		})
}

func (suite *DeployFunctionTestSuite) TestFunctionImageNameInStatus() {

	functionName := "some-test-function"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	// deploy function
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		suite.Require().NotNil(deployResult)

		// make sure deployment status contains image name
		suite.Require().NotEmpty(deployResult.FunctionStatus.ContainerImage)

		// get the function
		function := suite.GetFunction(&platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})

		status := function.GetStatus()

		// make sure the image name exists in the function status
		suite.Require().NotEmpty(status.ContainerImage)

		suite.Require().Equal(deployResult.FunctionStatus.ContainerImage, status.ContainerImage)

		return true
	})
}

func (suite *DeployFunctionTestSuite) TestFunctionSecretCreation() {
	scrubber := functionconfig.NewScrubber(nil, nil)

	functionName := "func-with-secret"
	password := "1234"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	// set platform config to support scrubbing
	suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = true

	// reset platform configuration when done
	defer func() {
		suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = false
	}()

	// add sensitive fields
	createFunctionOptions.FunctionConfig.Spec.Build.CodeEntryAttributes = map[string]interface{}{
		"password": password,
	}

	// deploy function
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// get function secrets
		secrets, err := suite.Platform.GetFunctionSecrets(suite.Ctx, functionName, suite.Namespace)
		suite.Require().NoError(err)
		suite.Require().Len(secrets, 1)

		for _, secret := range secrets {
			secret := secret.Kubernetes
			if !strings.HasPrefix(secret.Name, functionconfig.NuclioFlexVolumeSecretNamePrefix) {

				// decode data from secret
				decodedSecretData, err := scrubber.DecodeSecretData(secret.Data)
				suite.Require().NoError(err)
				suite.Logger.DebugWithCtx(suite.Ctx,
					"Got function secret",
					"secretData", secret.Data,
					"decodedSecretData", decodedSecretData)

				// verify password is in secret data
				secretKey := strings.ToLower("$ref:/spec/build/codeEntryAttributes/password")
				suite.Require().Equal(password, decodedSecretData[secretKey])

				// verify secret's "content" also contains the password
				secretContent := string(secret.Data["content"])
				decodedContents, err := scrubber.DecodeSecretsMapContent(secretContent)
				suite.Require().NoError(err)

				suite.Logger.DebugWithCtx(suite.Ctx,
					"Decoded secret data content",
					"decodedSecretsDataContent", decodedContents)

				suite.Require().Equal(password, decodedContents[secretKey])

			} else {

				suite.Logger.DebugWithCtx(suite.Ctx,
					"Got unknown secret",
					"secretName", secret.Name,
					"secretData", secret.Data)
				suite.Failf("Got unknown secret", "Secret name: %s", secret.Name)
			}

		}
		return true
	})
}

func (suite *DeployFunctionTestSuite) TestSecretEnvVarNotPresent() {
	functionName := "regulart-func"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	// set platform config to support scrubbing
	suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = true

	// reset platform configuration when done
	defer func() {
		suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = false
	}()

	// deploy function
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		suite.Require().NotNil(deployResult)

		// get the function
		function := suite.GetFunction(&platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})

		// validate secret restoration env var is not in spec
		restoreSecretEnvVar := v1.EnvVar{
			Name:  common.RestoreConfigFromSecretEnvVar,
			Value: "true",
		}
		suite.Require().False(common.EnvInSlice(restoreSecretEnvVar, function.GetConfig().Spec.Env))

		return true
	})
}

func (suite *DeployFunctionTestSuite) TestMultipleVolumeSecrets() {
	scrubber := functionconfig.NewScrubber(nil, nil)

	functionName := "func-with-multiple-volumes"
	accessKey := "1234"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	functionSecretCounter, volumeSecretCounter := 0, 0

	// set platform config to support scrubbing
	suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = true

	// reset platform configuration when done
	defer func() {
		suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = false
	}()

	// add sensitive fields
	createFunctionOptions.FunctionConfig.Spec.Volumes = []functionconfig.Volume{
		{
			Volume: v1.Volume{
				Name: "volume1",
				VolumeSource: v1.VolumeSource{
					FlexVolume: &v1.FlexVolumeSource{
						Driver: "v3io/fuse",
						Options: map[string]string{
							"accessKey": accessKey,
							"subPath":   "/sub1",
							"container": "some-container",
						},
					},
				},
			},
			VolumeMount: v1.VolumeMount{
				Name:      "volume1",
				MountPath: "/tmp/volume1",
			},
		},
		{
			Volume: v1.Volume{
				Name: "volume2",
				VolumeSource: v1.VolumeSource{
					FlexVolume: &v1.FlexVolumeSource{
						Driver: "v3io/fuse",
						Options: map[string]string{
							"accessKey": accessKey,
							"subPath":   "/sub1",
							"container": "some-container",
						},
					},
				},
			},
			VolumeMount: v1.VolumeMount{
				Name:      "volume2",
				MountPath: "/tmp/volume2",
			},
		},
	}

	// deploy function, we expect a failure due to the v3io access key being a dummy value
	suite.DeployFunctionExpectError(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool { // nolint: errcheck

		// get function secrets
		secrets, err := suite.Platform.GetFunctionSecrets(suite.Ctx, functionName, suite.Namespace)
		suite.Require().NoError(err)
		suite.Require().Len(secrets, 3)

		for _, secret := range secrets {
			kubeSecret := secret.Kubernetes
			switch {
			case !strings.HasPrefix(kubeSecret.Name, functionconfig.NuclioFlexVolumeSecretNamePrefix):
				functionSecretCounter++

				// decode data from secret
				decodedSecretData, err := scrubber.DecodeSecretData(kubeSecret.Data)
				suite.Require().NoError(err)
				suite.Logger.DebugWithCtx(suite.Ctx,
					"Got function secret",
					"secretData", kubeSecret.Data,
					"decodedSecretData", decodedSecretData)

				// verify accessKey is in secret data
				for key, value := range decodedSecretData {
					if strings.Contains(key, "accesskey") {
						suite.Require().Equal(accessKey, value)
					}
				}

			case strings.HasPrefix(kubeSecret.Name, functionconfig.NuclioFlexVolumeSecretNamePrefix):
				volumeSecretCounter++

				// validate that the secret contains the volume label
				volumeNameLabel, exists := kubeSecret.Labels[common.NuclioResourceLabelKeyVolumeName]
				suite.Require().True(exists)
				suite.Require().Contains([]string{"volume1", "volume2"}, volumeNameLabel)

				// validate that the secret contains the access key
				accessKeyData, exists := kubeSecret.Data["accessKey"]
				suite.Require().True(exists)

				suite.Require().Equal(accessKey, string(accessKeyData))

			default:
				suite.Logger.DebugWithCtx(suite.Ctx,
					"Got unknown secret",
					"secretName", kubeSecret.Name,
					"secretData", kubeSecret.Data)
				suite.Failf("Got unknown secret.", "Secret name: %s", kubeSecret.Name)
			}

		}
		return true
	})

	suite.Require().Equal(1, functionSecretCounter)
	suite.Require().Equal(2, volumeSecretCounter)
}

func (suite *DeployFunctionTestSuite) TestRedeployFunctionWithScrubbedField() {
	scrubber := functionconfig.NewScrubber(nil, nil)

	functionName := "func-with-v3io-stream-trigger"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	// set platform config to support scrubbing
	suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = true

	// reset platform configuration when done
	defer func() {
		suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = false
	}()

	firstPassword := "1234"
	secondPassword := "abcd"
	passwordPath := "$ref:/spec/build/codeentryattributes/password"

	validateSecretPasswordFunc := func(password string) {

		// get function secret
		secrets, err := suite.Platform.GetFunctionSecrets(suite.Ctx, functionName, suite.Namespace)
		suite.Require().NoError(err)
		suite.Require().Len(secrets, 1)

		// decode secret data
		decodedContents, err := scrubber.DecodeSecretData(secrets[0].Kubernetes.Data)
		suite.Require().NoError(err)

		// make sure first password is in secret
		suite.Require().Equal(password, decodedContents[passwordPath])
	}

	// add sensitive fields
	createFunctionOptions.FunctionConfig.Spec.Build.CodeEntryAttributes = map[string]interface{}{
		"password": firstPassword,
	}

	// deploy function
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		suite.Require().NotNil(deployResult)

		// validate first password
		validateSecretPasswordFunc(firstPassword)

		// change password
		newCreateFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
		newCreateFunctionOptions.FunctionConfig.Spec.Build.CodeEntryAttributes = map[string]interface{}{
			"password": secondPassword,
		}

		// redeploy function
		suite.DeployFunction(newCreateFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

			// make sure deployment succeeded
			suite.Require().NotNil(deployResult)

			validateSecretPasswordFunc(secondPassword)

			return true
		})

		return true
	})
}

func (suite *DeployFunctionTestSuite) TestCleanFlexVolumeSubPath() {
	functionName := "func-with-v3io-fuse-volume"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	createFunctionOptions.FunctionConfig.Spec.Volumes = []functionconfig.Volume{
		{
			Volume: v1.Volume{
				Name: "volume1",
				VolumeSource: v1.VolumeSource{
					FlexVolume: &v1.FlexVolumeSource{
						Driver: "v3io/fuse",
						Options: map[string]string{
							"subPath":   "///some/bad/sub/path//",
							"container": "some-container",
							"accessKey": "some-access-key",
						},
					},
				},
			},
			VolumeMount: v1.VolumeMount{
				Name:      "volume1",
				MountPath: "/tmp/volume1",
			},
		},
	}

	// deploy function, we expect a failure due to the v3io access key being a dummy value
	_, err := suite.DeployFunctionExpectError(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		function := suite.GetFunction(&platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})

		suite.Require().Equal("/some/bad/sub/path",
			function.GetConfig().Spec.Volumes[0].Volume.FlexVolume.Options["subPath"])
		return true
	})
	suite.Require().Error(err)
}

func (suite *DeployFunctionTestSuite) TestRedeployWithReplicasAndSecret() {
	one := 1
	four := 4

	// set platform config to support scrubbing
	suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = true

	// reset platform configuration when done
	defer func() {
		suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = false
	}()

	// create function with 1 replica
	functionName := "my-function"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.MinReplicas = &one
	createFunctionOptions.FunctionConfig.Spec.MaxReplicas = &one

	suite.Logger.InfoWith("Deploying function without sensitive data",
		"functionName", functionName,
		"replicas", one)

	// use suite.DeployFunctionAndRedeploy
	suite.DeployFunctionAndRedeploy(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult)

		// redeploy function with 4 replicas, and a sensitive field
		createFunctionOptions.FunctionConfig.Spec.MinReplicas = &four
		createFunctionOptions.FunctionConfig.Spec.MaxReplicas = &four

		// add sensitive field
		createFunctionOptions.FunctionConfig.Spec.Build.CodeEntryAttributes = map[string]interface{}{
			"password": "my-password",
		}

		// function will try to read the secret, and will fail if the secret is not mounted
		createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.
			EncodeToString([]byte(fmt.Sprintf(`
def init_context(context):
    context.logger.info("Init context - reading secret")
    secret_content = open("%s", "r").read()

def handler(context, event):
    context.logger.info("Hello world")
`, path.Join(functionconfig.FunctionSecretMountPath, functionconfig.SecretContentKey))))

		suite.Logger.InfoWith("Redeploying function with sensitive data",
			"functionName", functionName,
			"replicas", four)

		return true
	}, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult)

		// make sure function has 4 replicas
		function := suite.GetFunction(&platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})
		suite.Require().Equal(four, *function.GetConfig().Spec.MinReplicas)
		suite.Require().Equal(four, *function.GetConfig().Spec.MaxReplicas)

		return true
	})
}

func (suite *DeployFunctionTestSuite) TestRedeployWithReplicasAndValidateResources() {
	// set platform config to support scrubbing
	suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = true

	// reset platform configuration when done
	defer func() {
		suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = false
	}()

	one := 1
	two := 2
	createFunctionOptions := suite.CompileCreateFunctionOptions("my-function")
	createFunctionOptions.FunctionConfig.Spec.MinReplicas = &one

	suite.Logger.InfoWith("Deploying function with 1 replica",
		"functionName", createFunctionOptions.FunctionConfig.Meta.Name)

	// deploy function with 1 replica
	suite.DeployFunctionAndRedeploy(createFunctionOptions, func(firstDeployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(firstDeployResult)

		suite.Logger.InfoWith("Redeploying function with 2 replica",
			"functionName", createFunctionOptions.FunctionConfig.Meta.Name)

		// change replicas to 2
		createFunctionOptions.FunctionConfig.Spec.MaxReplicas = &two

		return true
	}, func(secondDeployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(secondDeployResult)

		// validate function resources are not zero (meaning they were updated with defaults)
		function := suite.GetFunction(&platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})

		functionSpec := function.GetConfig().Spec

		// only resource requests are enriched by default
		suite.Require().NotNil(functionSpec.Resources)
		suite.Require().NotNil(functionSpec.Resources.Requests)
		suite.Require().NotZero(functionSpec.Resources.Requests.Cpu().MilliValue())
		suite.Require().NotZero(functionSpec.Resources.Requests.Memory().Value())

		return true
	})
}

func (suite *DeployFunctionTestSuite) TestCreateFunctionWithCustomScalingMetrics() {
	one := 1
	four := 4
	eighty := resource.MustParse("80")
	gpuMetricName := "DCGM_FI_DEV_GPU_UTIL"
	functionName := "func-with-custom-scaling-metrics"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.MinReplicas = &one
	createFunctionOptions.FunctionConfig.Spec.MaxReplicas = &four
	createFunctionOptions.FunctionConfig.Spec.CustomScalingMetricSpecs = []autosv2.MetricSpec{
		{
			Type: autosv2.PodsMetricSourceType,
			Pods: &autosv2.PodsMetricSource{
				Metric: autosv2.MetricIdentifier{
					Name: gpuMetricName,
				},
				Target: autosv2.MetricTarget{
					Type:         autosv2.AverageValueMetricType,
					AverageValue: &eighty,
				},
			},
		},
	}

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult)

		// get the function's HPA and validate it has the custom scaling metrics
		hpaName := kube.HPANameFromFunctionName(functionName)
		hpa, err := suite.KubeClientSet.AutoscalingV2().HorizontalPodAutoscalers(suite.Namespace).Get(suite.Ctx,
			hpaName, metav1.GetOptions{})
		suite.Require().NoError(err)

		suite.Require().Len(hpa.Spec.Metrics, 1)
		suite.Require().Equal(autosv2.PodsMetricSourceType, hpa.Spec.Metrics[0].Type)
		suite.Require().Equal(gpuMetricName, hpa.Spec.Metrics[0].Pods.Metric.Name)
		suite.Require().Equal(&eighty, hpa.Spec.Metrics[0].Pods.Target.AverageValue)

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
		createAPIGatewayOptions := suite.CompileCreateAPIGatewayOptions(apiGatewayName, functionName)
		err := suite.DeployAPIGateway(createAPIGatewayOptions, func(ingress *networkingv1.Ingress) {
			suite.Assert().Contains(ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Name, functionName)

			// try to delete the function while it uses this api gateway
			err := suite.Platform.DeleteFunction(suite.Ctx, &platform.DeleteFunctionOptions{
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
		err := suite.Platform.DeleteFunction(suite.Ctx, &platform.DeleteFunctionOptions{
			FunctionConfig: deployResult.UpdatedFunctionConfig,
		})
		suite.Require().Error(err)

		deployResult.UpdatedFunctionConfig.Meta.ResourceVersion = function.GetConfig().Meta.ResourceVersion

		// succeeded delete function
		suite.Logger.Info("Deleting function")
		err = suite.Platform.DeleteFunction(suite.Ctx, &platform.DeleteFunctionOptions{
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
	ctx := suite.Ctx

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
	_, err := suite.Platform.CreateFunction(ctx, createFunctionOptions)
	suite.Require().NoError(err, "Failed to create function")

	// delete leftovers
	defer func() {
		err = suite.Platform.DeleteFunction(ctx, &platform.DeleteFunctionOptions{
			FunctionConfig: createFunctionOptions.FunctionConfig,
		})
		suite.Require().NoError(err, "Failed to delete function")
	}()

	// change annotations
	createFunctionOptions.FunctionConfig.Meta.Annotations["annotation-key"] = "annotation-value-changed"
	createFunctionOptions.FunctionConfig.Meta.Annotations["added-annotation"] = "added"

	// update function
	err = suite.Platform.UpdateFunction(ctx, &platform.UpdateFunctionOptions{
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
					"Image", "ImageHash", "Resources"), // auto generated during deploy

				// TODO: compare triggers as well
				// currently block due to serviceType being converted to string during get functions)
				cmpopts.IgnoreTypes(map[string]functionconfig.Trigger{}),
			},
		))
}

func (suite *UpdateFunctionTestSuite) TestUpdateFunctionWithSecret() {
	ctx := suite.Ctx
	functionName := "update-with-secret"
	password := "1234"
	secretPasswordKey := fmt.Sprintf("%s%s",
		functionconfig.ReferencePrefix,
		"/spec/build/codeentryattributes/password")

	// set platform config to support scrubbing
	suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = true

	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	// add sensitive fields
	createFunctionOptions.FunctionConfig.Spec.Build.CodeEntryAttributes = map[string]interface{}{
		"password": password,
	}

	// create function
	_, err := suite.Platform.CreateFunction(ctx, createFunctionOptions)
	suite.Require().NoError(err, "Failed to create function")

	// delete leftovers and reset platform configuration when done
	defer func() {
		suite.PlatformConfiguration.SensitiveFields.MaskSensitiveFields = false

		err = suite.Platform.DeleteFunction(ctx, &platform.DeleteFunctionOptions{
			FunctionConfig: createFunctionOptions.FunctionConfig,
		})
		suite.Require().NoError(err, "Failed to delete function")
	}()

	// get function secret data
	secretData, err := suite.Platform.GetFunctionSecretMap(ctx, functionName, suite.Namespace)
	suite.Require().NoError(err, "Failed to get function secret data")

	// ensure secret contains the password
	suite.Require().Equal(password, secretData[secretPasswordKey])

	// set the password to the reference, to mimic updating with an existing secret
	createFunctionOptions.FunctionConfig.Spec.Build.CodeEntryAttributes["password"] = secretPasswordKey

	// update function - use 'CreateFunction' since 'UpdateFunction' doesn't support updating secrets
	_, err = suite.Platform.CreateFunction(ctx, createFunctionOptions)
	suite.Require().NoError(err, "Failed to create function")

	// get function secret data
	secretData, err = suite.Platform.GetFunctionSecretMap(ctx, functionName, suite.Namespace)
	suite.Require().NoError(err, "Failed to get function secret data")

	// ensure secret still contains the same password
	suite.Require().Equal(password, secretData[secretPasswordKey])
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
		createAPIGatewayOptions := suite.CompileCreateAPIGatewayOptions(apiGatewayName, functionName)
		createAPIGatewayOptions.APIGatewayConfig.Spec.AuthenticationMode = ingress.AuthenticationModeOauth2
		err := suite.DeployAPIGateway(createAPIGatewayOptions, func(ingress *networkingv1.Ingress) {
			suite.Require().NotContains(ingress.Annotations, "nginx.ingress.kubernetes.io/auth-signin")
			suite.Require().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/auth-url"], configOauth2ProxyURL)
			suite.Require().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/configuration-snippet"],
				fmt.Sprintf(`proxy_set_header X-Nuclio-Target "%s";`, functionName))
		})
		suite.Require().NoError(err)

		overrideOauth2ProxyURL := "override-oauth2-url"
		createAPIGatewayOptions = suite.CompileCreateAPIGatewayOptions(apiGatewayName, functionName)
		createAPIGatewayOptions.APIGatewayConfig.Spec.AuthenticationMode = ingress.AuthenticationModeOauth2
		createAPIGatewayOptions.APIGatewayConfig.Spec.Authentication = &platform.APIGatewayAuthenticationSpec{
			DexAuth: &ingress.DexAuth{
				Oauth2ProxyURL:               overrideOauth2ProxyURL,
				RedirectUnauthorizedToSignIn: true,
			},
		}
		err = suite.DeployAPIGateway(createAPIGatewayOptions, func(ingress *networkingv1.Ingress) {
			suite.Assert().Contains(ingress.Annotations, "nginx.ingress.kubernetes.io/auth-signin")
			suite.Assert().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/auth-signin"], overrideOauth2ProxyURL)
			suite.Assert().Contains(ingress.Annotations["nginx.ingress.kubernetes.io/auth-url"], overrideOauth2ProxyURL)
		})
		suite.Require().NoError(err)

		return true
	})
}

func (suite *DeployAPIGatewayTestSuite) TestUpdate() {
	projectName := "some-project-" + xid.New().String()
	functionName := "function-name-" + xid.New().String()
	apiGatewayName := "apigw-name-" + xid.New().String()
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	// create project
	err := suite.Platform.CreateProject(suite.Ctx, &platform.CreateProjectOptions{
		ProjectConfig: &platform.ProjectConfig{
			Meta: platform.ProjectMeta{
				Name:      projectName,
				Namespace: suite.Namespace,
			},
		},
	})
	suite.Require().NoError(err, "Failed to create project")
	createFunctionOptions.FunctionConfig.Meta.Labels["nuclio.io/project-name"] = projectName

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		createAPIGatewayOptions := suite.CompileCreateAPIGatewayOptions(apiGatewayName, functionName)
		beforeUpdateHostValue := "before-update-host.com"
		createAPIGatewayOptions.APIGatewayConfig.Spec.Host = beforeUpdateHostValue
		createAPIGatewayOptions.APIGatewayConfig.Meta.Labels["nuclio.io/project-name"] = projectName
		createAPIGatewayOptions.APIGatewayConfig.Meta.Annotations = map[string]string{
			"some/annotation": "some-value",
		}

		// create
		err := suite.Platform.CreateAPIGateway(suite.Ctx, createAPIGatewayOptions)
		suite.Require().NoError(err)

		// delete leftovers
		defer suite.Platform.DeleteAPIGateway(suite.Ctx, &platform.DeleteAPIGatewayOptions{ // nolint: errcheck
			Meta: createAPIGatewayOptions.APIGatewayConfig.Meta,
		})

		suite.WaitForAPIGatewayState(&platform.GetAPIGatewaysOptions{
			Name:      createAPIGatewayOptions.APIGatewayConfig.Meta.Name,
			Namespace: createAPIGatewayOptions.APIGatewayConfig.Meta.Namespace,
		}, platform.APIGatewayStateReady, 10*time.Second)

		ingressInstance := suite.GetAPIGatewayIngress(createAPIGatewayOptions.APIGatewayConfig.Meta.Name, false)
		suite.Require().Equal(beforeUpdateHostValue, ingressInstance.Spec.Rules[0].Host)

		// ensure ingress labels were created correctly
		suite.Require().Equal("apigateway", ingressInstance.Labels["nuclio.io/class"])
		suite.Require().Equal("ingress-manager", ingressInstance.Labels["nuclio.io/app"])
		suite.Require().Equal(apiGatewayName, ingressInstance.Labels["nuclio.io/apigateway-name"])
		suite.Require().Equal(projectName, ingressInstance.Labels["nuclio.io/project-name"])
		suite.Require().Equal("some-value", ingressInstance.Annotations["some/annotation"])

		// change host, update
		afterUpdateHostValue := "after-update-host.com"
		annotations := map[string]string{
			"annotation-key": "annotation-value",
		}
		createAPIGatewayOptions.APIGatewayConfig.Spec.Host = afterUpdateHostValue
		createAPIGatewayOptions.APIGatewayConfig.Meta.Annotations = annotations
		err = suite.Platform.UpdateAPIGateway(suite.Ctx, &platform.UpdateAPIGatewayOptions{
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
	err := suite.Platform.CreateProject(suite.Ctx, &platform.CreateProjectOptions{
		ProjectConfig: &projectConfig,
	})
	suite.Require().NoError(err, "Failed to create project")
	defer func() {
		err = suite.Platform.DeleteProject(suite.Ctx, &platform.DeleteProjectOptions{
			Meta:     projectConfig.Meta,
			Strategy: platform.DeleteProjectStrategyRestricted,
		})
		suite.Require().NoError(err, "Failed to delete project")
	}()

	// get created project
	projects, err := suite.Platform.GetProjects(suite.Ctx, &platform.GetProjectsOptions{
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
	err := suite.Platform.CreateProject(suite.Ctx, &platform.CreateProjectOptions{
		ProjectConfig: &projectConfig,
	})
	suite.Require().NoError(err, "Failed to create project")

	// delete leftover
	defer func() {
		err := suite.Platform.DeleteProject(suite.Ctx, &platform.DeleteProjectOptions{
			Meta:     projectConfig.Meta,
			Strategy: platform.DeleteProjectStrategyRestricted,
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
	err = suite.Platform.UpdateProject(suite.Ctx, &platform.UpdateProjectOptions{
		ProjectConfig: projectConfig,
	})
	suite.Require().NoError(err, "Failed to update project")

	// get updated project
	updatedProject := suite.GetProject(&platform.GetProjectsOptions{
		Meta: projectConfig.Meta,
	})
	suite.Require().Empty(cmp.Diff(projectConfig, *updatedProject.GetConfig(),
		cmp.Options{
			cmpopts.IgnoreFields(projectConfig.Status, "UpdatedAt"), // automatically populated
		}))
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
	err := suite.Platform.CreateProject(suite.Ctx, &platform.CreateProjectOptions{
		ProjectConfig: &projectConfig,
	})
	suite.Require().NoError(err, "Failed to create project")

	// delete project
	err = suite.Platform.DeleteProject(suite.Ctx, &platform.DeleteProjectOptions{
		Meta:     projectConfig.Meta,
		Strategy: platform.DeleteProjectStrategyRestricted,
	})
	suite.Require().NoError(err, "Failed to delete project")

	// ensure project does not exist
	projects, err := suite.Platform.GetProjects(suite.Ctx, &platform.GetProjectsOptions{
		Meta: projectConfig.Meta,
	})
	suite.Require().NoError(err, "Failed to get projects")
	suite.Require().Equal(0, len(projects))
}

func (suite *ProjectTestSuite) TestDeleteCascading() {

	// create project
	projectToDeleteConfig := platform.ProjectConfig{
		Meta: platform.ProjectMeta{
			Name:      "project-to-delete",
			Namespace: suite.Namespace,
		},
	}
	err := suite.Platform.CreateProject(suite.Ctx, &platform.CreateProjectOptions{
		ProjectConfig: &projectToDeleteConfig,
	})
	suite.Require().NoError(err, "Failed to create project")

	// create 2 functions (deleted along with `projectToDeleteConfig`)

	// create function A
	functionToDeleteA := suite.CreateImportedFunction("func-to-delete-a", projectToDeleteConfig.Meta.Name)
	functionToDeleteB := suite.CreateImportedFunction("func-to-delete-b", projectToDeleteConfig.Meta.Name)

	// delete leftovers
	defer suite.Platform.DeleteFunction(suite.Ctx, &platform.DeleteFunctionOptions{ // nolint: errcheck
		FunctionConfig: *functionToDeleteA,
	})
	defer suite.Platform.DeleteFunction(suite.Ctx, &platform.DeleteFunctionOptions{ // nolint: errcheck
		FunctionConfig: *functionToDeleteB,
	})

	// create api gateway for function A (deleted along with `projectToDeleteConfig`)
	createAPIGatewayOptions := suite.CompileCreateAPIGatewayOptions("apigw-to-delete",
		functionToDeleteA.Meta.Name)
	createAPIGatewayOptions.APIGatewayConfig.Meta.Labels["nuclio.io/project-name"] = projectToDeleteConfig.Meta.Name
	err = suite.Platform.CreateAPIGateway(suite.Ctx, createAPIGatewayOptions)
	suite.Require().NoError(err)
	defer suite.Platform.DeleteAPIGateway(suite.Ctx, &platform.DeleteAPIGatewayOptions{ // nolint: errcheck
		Meta: createAPIGatewayOptions.APIGatewayConfig.Meta,
	})

	suite.WaitForAPIGatewayState(&platform.GetAPIGatewaysOptions{
		Name:      createAPIGatewayOptions.APIGatewayConfig.Meta.Name,
		Namespace: createAPIGatewayOptions.APIGatewayConfig.Meta.Namespace,
	}, platform.APIGatewayStateReady, 10*time.Second)

	// create 2 function events for function B (deleted along with `projectToDeleteConfig`)
	functionEventA := suite.CompileCreateFunctionEventOptions("function-event-a", functionToDeleteB.Meta.Name)
	err = suite.Platform.CreateFunctionEvent(suite.Ctx, functionEventA)
	suite.Require().NoError(err)
	defer suite.Platform.DeleteFunctionEvent(suite.Ctx, &platform.DeleteFunctionEventOptions{ // nolint: errcheck
		Meta: platform.FunctionEventMeta{
			Name:      functionEventA.FunctionEventConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
	})

	functionEventB := suite.CompileCreateFunctionEventOptions("function-event-b", functionToDeleteB.Meta.Name)
	err = suite.Platform.CreateFunctionEvent(suite.Ctx, functionEventB)
	suite.Require().NoError(err)
	defer suite.Platform.DeleteFunctionEvent(suite.Ctx, &platform.DeleteFunctionEventOptions{ // nolint: errcheck
		Meta: platform.FunctionEventMeta{
			Name:      functionEventB.FunctionEventConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
	})

	// try restrict - expect it to fail (project has sub resources)
	err = suite.Platform.DeleteProject(suite.Ctx, &platform.DeleteProjectOptions{
		Meta: platform.ProjectMeta{
			Name:      projectToDeleteConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
		Strategy: platform.DeleteProjectStrategyRestricted,
	})
	suite.Require().Error(err)

	// try cascading - expect it succeed
	err = suite.Platform.DeleteProject(suite.Ctx, &platform.DeleteProjectOptions{
		Meta: platform.ProjectMeta{
			Name:      projectToDeleteConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
		Strategy:                           platform.DeleteProjectStrategyCascading,
		WaitForResourcesDeletionCompletion: true,
		WaitForResourcesDeletionCompletionDuration: 3 * time.Minute,
	})
	suite.Require().NoError(err)

	// assertion - project should be deleted
	projects, err := suite.Platform.GetProjects(suite.Ctx, &platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      projectToDeleteConfig.Meta.Name,
			Namespace: suite.Namespace,
		},
	})
	suite.Require().NoError(err)
	suite.Require().Len(projects, 0)

	suite.Logger.InfoWith("Ensuring resources were removed (deletion is being executed in background")

	// ensure api gateway deleted
	apiGateways, err := suite.Platform.GetAPIGateways(suite.Ctx, &platform.GetAPIGatewaysOptions{
		Name:      createAPIGatewayOptions.APIGatewayConfig.Meta.Name,
		Namespace: suite.Namespace,
	})
	suite.Require().NoError(err)
	suite.Require().Len(apiGateways, 0, "Some api gateways were not removed")

	// ensure functions were deleted successfully
	for _, functionName := range []string{
		functionToDeleteA.Meta.Name,
		functionToDeleteB.Meta.Name,
	} {
		functions, err := suite.Platform.GetFunctions(suite.Ctx, &platform.GetFunctionsOptions{
			Name:      functionName,
			Namespace: suite.Namespace,
		})
		suite.Require().NoError(err)
		suite.Require().Len(functions, 0, "Some functions were not removed")
	}

	// ensure function events were deleted successfully
	for _, functionEventName := range []string{
		functionEventA.FunctionEventConfig.Meta.Name,
		functionEventB.FunctionEventConfig.Meta.Name,
	} {
		functionEvents, err := suite.Platform.GetFunctionEvents(suite.Ctx, &platform.GetFunctionEventsOptions{
			Meta: platform.FunctionEventMeta{
				Name:      functionEventName,
				Namespace: suite.Namespace,
			},
		})
		suite.Require().NoError(err)
		suite.Require().Len(functionEvents, 0, "Some function events were not removed")
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
