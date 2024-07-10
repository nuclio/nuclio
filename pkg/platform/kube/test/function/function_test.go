//go:build test_integration && test_kube && test_functions_kube

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

package function

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	suite2 "github.com/nuclio/nuclio/pkg/platform/kube/test/suite"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/processor/build/runtimeconfig"
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
	suite2.KubeTestSuite
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
			results, err := suite.ExecuteKubectl([]string{
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

func (suite *DeployFunctionTestSuite) TestDeployFunctionDisabledDefaultHttpTrigger() {
	createFunctionOptions := suite.CompileCreateFunctionOptions("disable-default-http")
	trueValue := true
	createFunctionOptions.FunctionConfig.Spec.DisableDefaultHTTPTrigger = &trueValue
	suite.DeployFunction(createFunctionOptions,

		// sanity
		func(deployResult *platform.CreateFunctionResult) bool {
			suite.Require().NotNil(deployResult)
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

func (suite *DeployFunctionTestSuite) TestDeployWithEnvFrom() {
	// saving runtime value to set it back after the test
	oldRuntime := suite.Platform.GetConfig().Runtime

	defer func() {
		_ = suite.KubeClientSet.CoreV1().Secrets(suite.Namespace).Delete(
			suite.Ctx,
			"test-platform",
			metav1.DeleteOptions{})
		_ = suite.KubeClientSet.CoreV1().Secrets(suite.Namespace).Delete(
			suite.Ctx,
			"test-function",
			metav1.DeleteOptions{})
		suite.Platform.GetConfig().Runtime = oldRuntime
	}()

	functionName := "test-env-from"
	_, err := suite.KubeClientSet.CoreV1().Secrets(suite.Namespace).Create(
		suite.Ctx, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: suite.Namespace, Name: "test-platform"},
			StringData: map[string]string{
				"SECRET_1": "platform",
				"SECRET_2": "platform"},
		},
		metav1.CreateOptions{})
	suite.Require().NoError(err, "Failed to create secret")
	_, err = suite.KubeClientSet.CoreV1().Secrets(suite.Namespace).Create(
		suite.Ctx, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: suite.Namespace, Name: "test-function"},
			StringData: map[string]string{
				"SECRET_2": "function"},
		},
		metav1.CreateOptions{})
	suite.Require().NoError(err, "Failed to create secret")
	suite.Platform.GetConfig().Runtime = &runtimeconfig.Config{
		Common: &runtimeconfig.Common{
			EnvFrom: []v1.EnvFromSource{{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{Name: "test-platform"}},
			},
			},
		},
	}
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.EnvFrom = []v1.EnvFromSource{{
		SecretRef: &v1.SecretEnvSource{LocalObjectReference: v1.LocalObjectReference{Name: "test-function"}},
	}}
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(`
import os
def handler(context, event):
  return "hello world"
def init_context(context):
  secret = os.getenv("SECRET_1")
  context.logger.info(f"secret1:{secret}")
  secret = os.getenv("SECRET_2")
  context.logger.info(f"secret2:{secret}")

`))
	suite.DeployFunction(createFunctionOptions,
		func(deployResult *platform.CreateFunctionResult) bool {
			suite.Require().NotNil(deployResult)
			pods := suite.GetFunctionPods(functionName)
			firstPod := pods[0]
			err := suite.WaitMessageInPodLog(firstPod.Namespace, firstPod.Name, "secret1:platform", &v1.PodLogOptions{}, 20*time.Second)
			suite.Require().NoError(err)
			err = suite.WaitMessageInPodLog(firstPod.Namespace, firstPod.Name, "secret2:function", &v1.PodLogOptions{}, 20*time.Second)
			suite.Require().NoError(err)
			return true
		})
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
		results, err := suite.ExecuteKubectl([]string{"exec", podName, "--", "id"}, nil)
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
	functionLabels := map[string]string{
		"my-function": "is-labeled",
	}
	suite.PlatformConfiguration.FunctionAugmentedConfigs = []platformconfig.LabelSelectorAndConfig{
		{
			LabelSelector: metav1.LabelSelector{
				MatchLabels: functionLabels,
			},
			FunctionConfig: functionconfig.Config{},
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
		suite.EnsureTriggerAmount(defaultTriggerFunctionName, "http", 1)
		defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
		return suite.VerifyCreatedTrigger(defaultTriggerFunctionName, defaultHTTPTrigger)
	})

	customTriggerFunctionName := "custom-http-trigger"
	createCustomTriggerFunctionOptions := suite.CompileCreateFunctionOptions(customTriggerFunctionName)
	customTrigger := functionconfig.Trigger{
		Kind:       "http",
		Name:       "custom-trigger",
		NumWorkers: 3,
	}
	createCustomTriggerFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		customTrigger.Name: customTrigger,
	}
	suite.DeployFunction(createCustomTriggerFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// ensure only 1 http trigger exists, always.
		suite.EnsureTriggerAmount(customTriggerFunctionName, "http", 1)
		return suite.VerifyCreatedTrigger(customTriggerFunctionName, customTrigger)
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
		NumWorkers: 1,
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
		NumWorkers: 1,
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

func (suite *DeployFunctionTestSuite) TestCreateFunctionWithIngress() {
	functionName := "func-with-ingress"
	ingressHost := "something.com"
	pathType := networkingv1.PathTypeImplementationSpecific
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"customTrigger": {
			Kind:       "http",
			Name:       "customTrigger",
			NumWorkers: 3,
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
			NumWorkers: 3,
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
	scrubber := suite.Platform.GetFunctionScrubber()

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
		secrets, err := scrubber.GetObjectSecrets(suite.Ctx, functionName, suite.Namespace)
		suite.Require().NoError(err)
		suite.Require().Len(secrets, 1)

		for _, secret := range secrets {
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
	scrubber := suite.Platform.GetFunctionScrubber()

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
		secrets, err := scrubber.GetObjectSecrets(suite.Ctx, functionName, suite.Namespace)
		suite.Require().NoError(err)
		suite.Require().Len(secrets, 3)

		for _, secret := range secrets {
			switch {
			case !strings.HasPrefix(secret.Name, functionconfig.NuclioFlexVolumeSecretNamePrefix):
				functionSecretCounter++

				// decode data from secret
				decodedSecretData, err := scrubber.DecodeSecretData(secret.Data)
				suite.Require().NoError(err)
				suite.Logger.DebugWithCtx(suite.Ctx,
					"Got function secret",
					"secretData", secret.Data,
					"decodedSecretData", decodedSecretData)

				// verify accessKey is in secret data
				for key, value := range decodedSecretData {
					if strings.Contains(key, "accesskey") {
						suite.Require().Equal(accessKey, value)
					}
				}

			case strings.HasPrefix(secret.Name, functionconfig.NuclioFlexVolumeSecretNamePrefix):
				volumeSecretCounter++

				// validate that the secret contains the volume label
				volumeNameLabel, exists := secret.Labels[common.NuclioResourceLabelKeyVolumeName]
				suite.Require().True(exists)
				suite.Require().Contains([]string{"volume1", "volume2"}, volumeNameLabel)

				// validate that the secret contains the access key
				accessKeyData, exists := secret.Data["accessKey"]
				suite.Require().True(exists)

				suite.Require().Equal(accessKey, string(accessKeyData))

			default:
				suite.Logger.DebugWithCtx(suite.Ctx,
					"Got unknown secret",
					"secretName", secret.Name,
					"secretData", secret.Data)
				suite.Failf("Got unknown secret.", "Secret name: %s", secret.Name)
			}

		}
		return true
	})

	suite.Require().Equal(1, functionSecretCounter)
	suite.Require().Equal(2, volumeSecretCounter)
}

func (suite *DeployFunctionTestSuite) TestRedeployFunctionWithScrubbedField() {
	scrubber := suite.Platform.GetFunctionScrubber()

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
		secrets, err := scrubber.GetObjectSecrets(suite.Ctx, functionName, suite.Namespace)
		suite.Require().NoError(err)
		suite.Require().Len(secrets, 1)

		// decode secret data
		decodedContents, err := scrubber.DecodeSecretData(secrets[0].Data)
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
	createFunctionOptions.FunctionConfig.Spec.ReadinessTimeoutSeconds = 10

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

func (suite *DeployFunctionTestSuite) TestDeployImportedFunctionAsScaledToZero() {
	zero := 0
	one := 1
	functionName := "func-scale-to-zero"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.MinReplicas = &zero
	createFunctionOptions.FunctionConfig.Spec.MaxReplicas = &one
	createFunctionOptions.FunctionConfig.AddPrevStateAnnotation(string(functionconfig.FunctionStateScaledToZero))
	result, deployErr := suite.Platform.CreateFunction(suite.Ctx, createFunctionOptions)
	suite.NoError(deployErr)
	suite.Require().Equal(result.FunctionStatus.State, functionconfig.FunctionStateScaledToZero)

	deployment, err := suite.KubeClientSet.AppsV1().Deployments("default").Get(suite.Ctx, "nuclio-func-scale-to-zero", metav1.GetOptions{})
	suite.NoError(err)
	suite.Require().Equal(int(*deployment.Spec.Replicas), 0)
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

func (suite *DeployFunctionTestSuite) TestDeployFunctionWithSidecarSanity() {
	functionName := "func-with-sidecar"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	sidecarContainerName := "sidecar-test"
	commands := []string{
		"sh",
		"-c",
		"for i in {1..10}; do echo $i; sleep 10; done; echo 'Done'",
	}

	// create a busybox sidecar
	createFunctionOptions.FunctionConfig.Spec.Sidecars = []*v1.Container{
		{
			Name:    sidecarContainerName,
			Image:   "busybox",
			Command: commands,
		},
	}

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult)

		// get the function pod and validate it has the sidecar
		pods := suite.GetFunctionPods(functionName)
		pod := pods[0]

		suite.Require().Len(pod.Spec.Containers, 2)
		suite.Require().Equal(sidecarContainerName, pod.Spec.Containers[1].Name)
		suite.Require().Equal("busybox", pod.Spec.Containers[1].Image)
		suite.Require().Equal(commands, pod.Spec.Containers[1].Command)

		// get the logs from the sidecar container to validate it ran
		podLogOpts := v1.PodLogOptions{
			Container: sidecarContainerName,
		}
		err := common.RetryUntilSuccessful(20*time.Second, 1*time.Second, func() bool {
			return suite.validatePodLogsContainData(pod.Name, &podLogOpts, []string{"Done"})
		})
		suite.Require().NoError(err)

		return true
	})
}

func (suite *DeployFunctionTestSuite) TestDeployFunctionWithInitContainers() {
	functionName := "func-with-init-containers"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	command := []string{
		"sh",
		"-c",
		"echo The init container is running! && sleep 60",
	}
	initContainerName := "init-container-test"
	createFunctionOptions.FunctionConfig.Spec.InitContainers = []*v1.Container{
		{
			Name:    initContainerName,
			Image:   "busybox",
			Command: command,
		},
	}

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult)

		// get the function pod and validate it has the init container
		pods := suite.GetFunctionPods(functionName)
		pod := pods[0]

		suite.Require().Len(pod.Spec.InitContainers, 1)

		suite.Require().Equal(initContainerName, pod.Spec.InitContainers[0].Name)
		suite.Require().Equal("busybox", pod.Spec.InitContainers[0].Image)
		suite.Require().Equal(command, pod.Spec.InitContainers[0].Command)

		// get the logs from the init container to validate it ran
		podLogOpts := v1.PodLogOptions{
			Container: initContainerName,
		}
		err := common.RetryUntilSuccessful(20*time.Second, 1*time.Second, func() bool {
			return suite.validatePodLogsContainData(pod.Name, &podLogOpts, []string{"The init container is running!"})
		})
		suite.Require().NoError(err)
		return true
	})
}

func (suite *DeployFunctionTestSuite) TestDeploySidecarEnrichment() {
	functionName := "func-with-enriched-sidecar"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	// create a temp dir to mount as a volume
	tempDir, err := os.MkdirTemp("", "nuclio-test-*")
	suite.Require().NoError(err)
	defer os.RemoveAll(tempDir) // nolint: errcheck

	// create a file in the temp dir
	tempFile, err := os.CreateTemp(tempDir, "nuclio-test-*")
	suite.Require().NoError(err)

	// write some data to the file
	data := "some data"
	_, err = tempFile.WriteString(data)
	suite.Require().NoError(err)

	fileName := "temp-file.txt"

	mountPath := "/tmp/volume1"
	createFunctionOptions.FunctionConfig.Spec.Volumes = []functionconfig.Volume{
		{
			Volume: v1.Volume{
				Name: "volume1",
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			},
			VolumeMount: v1.VolumeMount{
				Name:      "volume1",
				MountPath: mountPath,
			},
		},
	}
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(`
def init_context(context):
    context.logger.info("Init context - writing to file")
    with open("/tmp/volume1/temp-file.txt", "w") as f:
        f.write("some data")

def handler(context, event):
    return "Success"
`))

	createFunctionOptions.FunctionConfig.Spec.Resources = v1.ResourceRequirements{
		Limits: v1.ResourceList{
			"cpu":    resource.MustParse("100m"),
			"memory": resource.MustParse("128Mi"),
		},
	}
	envVarKey := "MY_ENV_VAR"
	envVarValue := "my-env-var-value"
	createFunctionOptions.FunctionConfig.Spec.Env = []v1.EnvVar{
		{
			Name:  envVarKey,
			Value: envVarValue,
		},
	}

	sidecarContainerName := "sidecar-test"
	commands := []string{
		"sh",
		"-c",
		// print the env var, wait for the file to be created, then print the file contents. lastly,
		// sleep for 20 seconds until the test is over
		fmt.Sprintf("echo $%[1]v; while [ ! -f \"%[2]v\" ]; do echo \"waiting for file\"; sleep 0.2; done; while true; do echo $(cat %[2]v); sleep 2; done",
			envVarKey, path.Join(mountPath, fileName)),
	}

	// create a busybox sidecar
	createFunctionOptions.FunctionConfig.Spec.Sidecars = []*v1.Container{
		{
			Name:    sidecarContainerName,
			Image:   "busybox",
			Command: commands,
		},
	}

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult)

		// give the function container a chance to write the file
		time.Sleep(3 * time.Second)

		// get the function pod and validate it has the sidecar
		pods, err := suite.KubeClientSet.CoreV1().Pods(suite.Namespace).List(suite.Ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", common.NuclioResourceLabelKeyFunctionName, functionName),
		})
		suite.Require().NoError(err)
		suite.Require().Len(pods.Items, 1)

		pod := pods.Items[0]
		suite.Require().Len(pod.Spec.Containers, 2)

		// get the logs from the sidecar container to validate it ran by checking the shared volume
		podLogOpts := v1.PodLogOptions{
			Container: sidecarContainerName,
		}
		containsLog := suite.validatePodLogsContainData(pod.Name, &podLogOpts, []string{data, envVarValue})
		suite.Require().Equal(containsLog, true)

		// validate the sidecar container has the same resource requests as the function
		suite.Require().Equal(pod.Spec.Containers[0].Resources.Requests, pod.Spec.Containers[1].Resources.Requests)

		return true
	})
}

func (suite *DeployFunctionTestSuite) TestRedeployWithUpdatedSidecarSpec() {
	functionName := "func-with-sidecar"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	sidecarContainerName := "sidecar-test"
	firstDeployCommands := []string{
		"sh",
		"-c",
		"for i in {1..10}; do echo $i; sleep 10; done; echo 'First deploy done'",
	}
	secondDeployCommands := []string{
		"/bin/sh",
		"-c",
		"for i in {1..10}; do echo $i; sleep 10; done; echo 'Second deploy done'",
	}

	busyboxImage := "busybox"
	alpineImage := "alpine"

	// create a busybox sidecar
	createFunctionOptions.FunctionConfig.Spec.Sidecars = []*v1.Container{
		{
			Name:    sidecarContainerName,
			Image:   busyboxImage,
			Command: firstDeployCommands,
		},
	}

	afterFirstDeploy := func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult)

		// get the function pod and validate it has the sidecar
		pods := suite.GetFunctionPods(functionName)
		pod := pods[0]

		suite.Require().Len(pod.Spec.Containers, 2)
		suite.Require().Equal(sidecarContainerName, pod.Spec.Containers[1].Name)
		suite.Require().Equal(busyboxImage, pod.Spec.Containers[1].Image)
		suite.Require().Equal(firstDeployCommands, pod.Spec.Containers[1].Command)

		// get the logs from the sidecar container to validate it ran
		podLogOpts := v1.PodLogOptions{
			Container: sidecarContainerName,
		}
		err := common.RetryUntilSuccessful(20*time.Second, 1*time.Second, func() bool {
			return suite.validatePodLogsContainData(pod.Name, &podLogOpts, []string{"First deploy done"})
		})
		suite.Require().NoError(err)

		// change the sidecar image and command
		createFunctionOptions.FunctionConfig.Spec.Sidecars = []*v1.Container{
			{
				Name:    sidecarContainerName,
				Image:   alpineImage,
				Command: secondDeployCommands,
			},
		}

		return true
	}

	afterSecondDeploy := func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult)

		// get the function pod and validate it has the sidecar
		pods := suite.GetFunctionPods(functionName)
		pod := pods[0]
		if len(pods) != 1 {
			// because the first pod might still be terminating, we search for the new pod according to the start time
			sort.Slice(pods, func(i, j int) bool {
				return pods[i].CreationTimestamp.Before(&pods[j].CreationTimestamp)
			})
			pod = pods[len(pods)-1]
		}

		// validate the sidecar container has the new image and command
		suite.Require().Len(pod.Spec.Containers, 2)
		suite.Require().Equal(sidecarContainerName, pod.Spec.Containers[1].Name)
		suite.Require().Equal(alpineImage, pod.Spec.Containers[1].Image)
		suite.Require().Equal(secondDeployCommands, pod.Spec.Containers[1].Command)

		// get the logs from the sidecar container to validate it ran the new command
		podLogOpts := v1.PodLogOptions{
			Container: sidecarContainerName,
		}
		err := common.RetryUntilSuccessful(20*time.Second, 1*time.Second, func() bool {
			return suite.validatePodLogsContainData(pod.Name, &podLogOpts, []string{"Second deploy done"})
		})
		suite.Require().NoError(err)

		return true
	}

	suite.DeployFunctionAndRedeploy(createFunctionOptions, afterFirstDeploy, afterSecondDeploy)
}

func (suite *DeployFunctionTestSuite) TestDeployFromGitSanity() {
	functionName := "func-from-git"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	createFunctionOptions.FunctionConfig.Spec.Handler = "main:Handler"
	createFunctionOptions.FunctionConfig.Spec.Runtime = "golang"

	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = ""
	createFunctionOptions.FunctionConfig.Spec.Build.CodeEntryType = build.GitEntryType
	createFunctionOptions.FunctionConfig.Spec.Build.Path = "https://github.com/sahare92/test-nuclio-cet.git"
	createFunctionOptions.FunctionConfig.Spec.Build.CodeEntryAttributes = map[string]interface{}{
		"branch":  "go-func",
		"workDir": "go-function/main.go",
	}

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotNil(deployResult)

		suite.Require().Empty(deployResult.UpdatedFunctionConfig.Spec.Build.FunctionSourceCode)

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

func (suite *DeployFunctionTestSuite) validatePodLogsContainData(podName string, options *v1.PodLogOptions, expectedData []string) bool {
	podLogRequest := suite.KubeClientSet.CoreV1().Pods(suite.Namespace).GetLogs(podName, options)
	podLogs, err := podLogRequest.Stream(suite.Ctx)
	suite.Require().NoError(err)
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	suite.Require().NoError(err)

	for _, data := range expectedData {
		if strings.Contains(buf.String(), data) {
			return true
		}
	}
	return false
}

type DeleteFunctionTestSuite struct {
	suite2.KubeTestSuite
}

func (suite *DeleteFunctionTestSuite) TestDeleteFunctionWhichHasApiGateway() {
	functionName := "func-to-delete"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		apiGatewayName := "func-apigw"
		createAPIGatewayOptions := suite.CompileCreateAPIGatewayOptions(apiGatewayName, functionName)
		err := suite.DeployAPIGateway(createAPIGatewayOptions, func(ingress *networkingv1.Ingress) {
			suite.Assert().Contains(ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Name, functionName)

			// try to delete the function while it uses this api gateway without DeleteApiGateways flag
			err := suite.Platform.DeleteFunction(suite.Ctx, &platform.DeleteFunctionOptions{
				FunctionConfig: createFunctionOptions.FunctionConfig,
			})
			// expect deletion error
			suite.Require().Equal(platform.ErrFunctionIsUsedByAPIGateways, errors.RootCause(err))

			// try to delete the function while it uses this api gateway with DeleteApiGateways flag
			err = suite.Platform.DeleteFunction(suite.Ctx, &platform.DeleteFunctionOptions{
				FunctionConfig:    createFunctionOptions.FunctionConfig,
				DeleteApiGateways: true,
			})
			// function should be deleted
			suite.Require().NoError(err)
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

func (suite *DeployFunctionTestSuite) TestProcessorShutdown() {
	functionName := "func-to-test-processor-shutdown"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	// change source code
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte(`
terminated = False
def handler(context, event):
    if terminated:
        context.logger.info("Got event after termination")
    else:
        context.logger.info("Got http event")
    return "got http event!"

def callback():
    import time
    time.sleep(3)
    terminated = True

def init_context(context):
    context.platform.set_termination_callback(callback)
`))

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.Require().NotEmpty(deployResult)
		pods := suite.GetFunctionPods(functionName)
		firstPod := pods[0]
		err := suite.WaitMessageInPodLog(firstPod.Namespace, firstPod.Name, "Processor started", &v1.PodLogOptions{}, 20*time.Second)
		suite.Require().NoError(err)

		// bombing function with requests to check that event won't be processed after worker termination
		ctx, cancel := context.WithCancel(suite.Ctx)
		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					suite.InvokeFunction("GET", deployResult.Port, "", nil)
				case <-ctx.Done():
					return
				}
			}
		}()

		err = suite.WaitMessageInPodLog(firstPod.Namespace, firstPod.Name, "Got http event", &v1.PodLogOptions{}, 20*time.Second)
		suite.Require().NoError(err)

		err = suite.KubeClientSet.CoreV1().Pods(firstPod.Namespace).Delete(suite.Ctx, firstPod.Name, metav1.DeleteOptions{})
		suite.Require().NoError(err)

		err = suite.WaitMessageInPodLog(firstPod.Namespace, firstPod.Name, "All triggers are terminated", &v1.PodLogOptions{}, 30*time.Second)
		suite.Require().NoError(err, "triggers were not terminated")
		cancel()

		err = suite.WaitMessageInPodLog(firstPod.Namespace, firstPod.Name, "Got event after termination", &v1.PodLogOptions{}, 20*time.Second)
		suite.Require().NotNil(err)
		return true
	})
}

type UpdateFunctionTestSuite struct {
	suite2.KubeTestSuite
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
	scrubber := functionconfig.NewScrubber(suite.Logger, suite.Platform.GetConfig().SensitiveFields.CompileSensitiveFieldsRegex(),
		suite.KubeClientSet)
	// get function secret data
	secretData, _, err := scrubber.GetObjectSecretMap(ctx, functionName, suite.Namespace)
	suite.Require().NoError(err, "Failed to get function secret data")

	// ensure secret contains the password
	suite.Require().Equal(password, secretData[secretPasswordKey])

	// set the password to the reference, to mimic updating with an existing secret
	createFunctionOptions.FunctionConfig.Spec.Build.CodeEntryAttributes["password"] = secretPasswordKey

	// update function - use 'CreateFunction' since 'UpdateFunction' doesn't support updating secrets
	_, err = suite.Platform.CreateFunction(ctx, createFunctionOptions)
	suite.Require().NoError(err, "Failed to create function")

	// get function secret data
	secretData, _, err = scrubber.GetObjectSecretMap(ctx, functionName, suite.Namespace)
	suite.Require().NoError(err, "Failed to get function secret data")

	// ensure secret still contains the same password
	suite.Require().Equal(password, secretData[secretPasswordKey])
}

func TestFunctionTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(DeployFunctionTestSuite))
	suite.Run(t, new(UpdateFunctionTestSuite))
	suite.Run(t, new(DeleteFunctionTestSuite))
}
