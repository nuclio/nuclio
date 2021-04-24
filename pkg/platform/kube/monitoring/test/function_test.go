// +build test_integration
// +build test_kube

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
	"strconv"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/monitoring"
	kubetest "github.com/nuclio/nuclio/pkg/platform/kube/test"

	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionMonitoringTestSuite struct {
	kubetest.KubeTestSuite
	oldPostDeploymentMonitoringBlockingInterval time.Duration
}

func (suite *FunctionMonitoringTestSuite) SetupSuite() {
	suite.KubeTestSuite.SetupSuite()

	// keep it for suite teardown
	suite.oldPostDeploymentMonitoringBlockingInterval = monitoring.PostDeploymentMonitoringBlockingInterval

	// decrease blocking interval, to make test run faster
	// give it ~5 seconds to recently deployed functions to stabilize, avoid transients
	monitoring.PostDeploymentMonitoringBlockingInterval = 5 * time.Second
}

func (suite *FunctionMonitoringTestSuite) TearDownSuite() {
	monitoring.PostDeploymentMonitoringBlockingInterval = suite.oldPostDeploymentMonitoringBlockingInterval
}

func (suite *FunctionMonitoringTestSuite) TestRecoverFromPodsHardLimit() {
	functionName := "function-pods-hard-limit"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	getFunctionOptions := &platform.GetFunctionsOptions{
		Name:      createFunctionOptions.FunctionConfig.Meta.Name,
		Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
	}
	zero := 0
	two := 2
	createFunctionOptions.FunctionConfig.Spec.Replicas = &two
	_ = suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// limit pod to zero
		resourceQuota, err := suite.KubeClientSet.
			CoreV1().
			ResourceQuotas(suite.Namespace).
			Create(&v1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nuclio-test-rq",
					Namespace: suite.Namespace,
				},
				Spec: v1.ResourceQuotaSpec{
					Hard: v1.ResourceList{
						v1.ResourcePods: resource.MustParse(strconv.Itoa(zero)),
					},
				},
				Status: v1.ResourceQuotaStatus{},
			})
		suite.Require().NoError(err)

		// clean leftovers
		defer suite.KubeClientSet.
			CoreV1().
			ResourceQuotas(suite.Namespace).
			Delete(resourceQuota.Name, &metav1.DeleteOptions{}) // nolint: errcheck

		// delete function pods
		suite.DeleteFunctionPods(functionName)

		// function becomes unhealthy, due to exceeding pods deployment hard limit
		suite.WaitForFunctionState(getFunctionOptions,
			functionconfig.FunctionStateUnhealthy,
			3*time.Minute)

		// increase hard limit, allowing the function reach its potential replicas
		resourceQuota.Spec.Hard = v1.ResourceList{
			v1.ResourcePods: resource.MustParse(strconv.Itoa(two)),
		}

		// remove to allow updating
		resourceQuota.ResourceVersion = ""
		suite.Logger.InfoWith("Updating resource quota",
			"podsResourceQuota", resourceQuota.Spec.Hard.Pods())
		_, err = suite.KubeClientSet.CoreV1().ResourceQuotas(suite.Namespace).Update(resourceQuota)
		suite.Require().NoError(err)

		// wait for function to become healthy again
		suite.WaitForFunctionState(getFunctionOptions,
			functionconfig.FunctionStateReady,
			3*time.Minute)
		return true
	})
}

func (suite *FunctionMonitoringTestSuite) TestNoRecoveryAfterBuildError() {
	functionName := "function-build-fail"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	getFunctionOptions := &platform.GetFunctionsOptions{
		Name:      createFunctionOptions.FunctionConfig.Meta.Name,
		Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
	}

	// wait for at least one monitor + post deployment blocking intervals
	postDeploymentSleepInterval := 2*suite.Controller.GetFunctionMonitoringInterval() +
		monitoring.PostDeploymentMonitoringBlockingInterval

	suite.DeployFunctionAndRedeployExpectError(createFunctionOptions,
		func(deployResult *platform.CreateFunctionResult) bool {

			// make next deploy fail
			createFunctionOptions.FunctionConfig.Spec.Build.Commands = []string{
				"exit 1",
			}
			return true
		},
		func(deployResult *platform.CreateFunctionResult) bool {

			// wait for monitoring
			time.Sleep(postDeploymentSleepInterval)

			// ensure function is still in error state (due to build error)
			suite.GetFunctionAndExpectState(getFunctionOptions, functionconfig.FunctionStateError)

			return true
		})
}

func (suite *FunctionMonitoringTestSuite) TestRecoveryAfterDeployError() {
	functionName := "function-recover-after-deploy-fail"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	getFunctionOptions := &platform.GetFunctionsOptions{
		Name:      createFunctionOptions.FunctionConfig.Meta.Name,
		Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
	}
	createFunctionOptions.FunctionConfig.Spec.ReadinessTimeoutSeconds = 10

	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-some-configmap",
		},
		Data: map[string]string{
			"data": "from test configmap",
		},
	}

	// delete created configmap traces
	defer suite.KubeClientSet.
		CoreV1().
		ConfigMaps(suite.Namespace).
		Delete(configMap.Name, &metav1.DeleteOptions{}) // nolint: errcheck

	functionVolume := functionconfig.Volume{
		Volume: v1.Volume{
			Name: "test-volume-name",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: configMap.Name,
					},
				},
			},
		},
		VolumeMount: v1.VolumeMount{
			Name:      "test-volume-name",
			MountPath: "/my/configmap",
		},
	}

	createFunctionOptions.FunctionConfig.Spec.Volumes = []functionconfig.Volume{functionVolume}

	// function will read and return the volumized configmap contents
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.
		EncodeToString([]byte(fmt.Sprintf(`
def handler(context, event):
  return open("%s/data", "r").read()
`, functionVolume.VolumeMount.MountPath)))

	// wait for at least one monitor + post deployment blocking intervals
	postDeploymentSleepInterval := 2*suite.Controller.GetFunctionMonitoringInterval() +
		monitoring.PostDeploymentMonitoringBlockingInterval

	_, err := suite.DeployFunctionExpectError(createFunctionOptions,
		func(deployResult *platform.CreateFunctionResult) bool {
			var err error

			// wait for monitoring
			time.Sleep(postDeploymentSleepInterval)

			// function would become unhealthy as its function deployment is missing the mentioned configmap
			suite.GetFunctionAndExpectState(getFunctionOptions, functionconfig.FunctionStateUnhealthy)

			// create the missing configmap
			configMap, err = suite.KubeClientSet.CoreV1().ConfigMaps(suite.Namespace).Create(configMap)
			suite.Require().NoError(err, "Failed to create configmap")

			// wait for k8s to recover deployment from missing configmap error
			suite.WaitForFunctionDeployment(functionName,
				3*time.Minute,
				func(functionDeployment *appsv1.Deployment) bool {
					suite.Logger.InfoWith("Waiting for deployment unavailable replicas to be zero",
						"unavailableReplicas", functionDeployment.Status.UnavailableReplicas)

					// wait until all replicas are available
					return functionDeployment.Status.UnavailableReplicas == 0
				})

			// wait for monitoring
			time.Sleep(postDeploymentSleepInterval)

			// function should be recovered by function monitoring
			suite.GetFunctionAndExpectState(getFunctionOptions, functionconfig.FunctionStateReady)

			return true
		})
	suite.Require().Error(err)
}

func (suite *FunctionMonitoringTestSuite) TestNoRecoveryAfterDeployError() {
	functionName := "function-no-recover-after-deploy-fail"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	getFunctionOptions := &platform.GetFunctionsOptions{
		Name:      createFunctionOptions.FunctionConfig.Meta.Name,
		Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
	}
	createFunctionOptions.FunctionConfig.Spec.ReadinessTimeoutSeconds = 10

	// function deploy would fail trying to run, leaving the function in error state forever
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.
		EncodeToString([]byte(`def invalidhandler(context, event):return ""`))

	// wait for at least one monitor + post deployment blocking intervals
	postDeploymentSleepInterval := 2*suite.Controller.GetFunctionMonitoringInterval() +
		monitoring.PostDeploymentMonitoringBlockingInterval

	_, err := suite.DeployFunctionExpectError(createFunctionOptions,
		func(deployResult *platform.CreateFunctionResult) bool {

			// wait for monitoring
			time.Sleep(postDeploymentSleepInterval)

			// ensure function is still in error state (due to deploy error of missing configmap)
			suite.GetFunctionAndExpectState(getFunctionOptions, functionconfig.FunctionStateError)

			// let function monitoring run for a while
			time.Sleep(postDeploymentSleepInterval)

			// function should be remained in error state
			suite.GetFunctionAndExpectState(getFunctionOptions, functionconfig.FunctionStateError)
			return true
		})
	suite.Require().Error(err)
}

func (suite *FunctionMonitoringTestSuite) TestRecoverErrorStateFunctionWhenResourcesAvailable() {
	functionName := "function-recovery"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	getFunctionOptions := &platform.GetFunctionsOptions{
		Name:      createFunctionOptions.FunctionConfig.Meta.Name,
		Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
	}

	functionMonitoringSleepTimeout := 2*suite.Controller.GetFunctionMonitoringInterval() +
		monitoring.PostDeploymentMonitoringBlockingInterval
	suite.DeployFunction(createFunctionOptions, func(deployResults *platform.CreateFunctionResult) bool {

		// get the function
		function := suite.GetFunction(getFunctionOptions)

		// ensure function is ready
		suite.Require().Equal(functionconfig.FunctionStateReady, function.GetStatus().State)

		// get function pod, first one is enough
		pod := suite.GetFunctionPods(functionName)[0]

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

		// wait for controller to mark function in error due to pods being unschedulable
		suite.WaitForFunctionState(getFunctionOptions,
			functionconfig.FunctionStateUnhealthy,
			functionMonitoringSleepTimeout)

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
		suite.WaitForFunctionPods(functionName, time.Minute, func(pods []v1.Pod) bool {
			suite.Logger.InfoWith("Waiting for function pods",
				"pods", pods,
				"expectedPodPhase", v1.PodRunning)
			for _, pod := range pods {
				if pod.Status.Phase != v1.PodRunning {
					return false
				}
			}
			return true
		})

		// wait for function state to become ready again
		suite.WaitForFunctionState(getFunctionOptions,
			functionconfig.FunctionStateReady,
			functionMonitoringSleepTimeout)
		return true
	})
}

func (suite *FunctionMonitoringTestSuite) TestPausedFunctionShouldRemainInReadyState() {
	functionName := "paused-function"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	getFunctionOptions := &platform.GetFunctionsOptions{
		Name:      createFunctionOptions.FunctionConfig.Meta.Name,
		Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
	}

	functionMonitoringSleepTimeout := 2 * suite.Controller.GetFunctionMonitoringInterval()
	suite.DeployFunction(createFunctionOptions, func(deployResults *platform.CreateFunctionResult) bool {
		zero := 0
		deployResults.UpdatedFunctionConfig.Spec.Replicas = &zero
		deployResults.UpdatedFunctionConfig.Spec.Disable = true
		deployResults.UpdatedFunctionConfig.Spec.Image = deployResults.Image
		err := suite.Platform.UpdateFunction(&platform.UpdateFunctionOptions{
			FunctionMeta: &deployResults.UpdatedFunctionConfig.Meta,
			FunctionSpec: &deployResults.UpdatedFunctionConfig.Spec,
		})
		suite.Require().NoError(err, "Failed to update function")

		// wait for function deployment replicas gets to zero
		suite.WaitForFunctionDeployment(functionName,
			1*time.Minute,
			func(functionDeployment *appsv1.Deployment) bool {
				suite.Logger.InfoWith("Waiting for deployment replicas to be zero",
					"replicas", functionDeployment.Status.Replicas)
				return functionDeployment.Status.Replicas == 0
			})

		// wait for function monitoring to run
		time.Sleep(functionMonitoringSleepTimeout)

		// get the function
		function := suite.GetFunction(getFunctionOptions)

		// function is ready, with 0 replicas
		suite.Require().Equal(functionconfig.FunctionStateReady, function.GetStatus().State)
		suite.Require().NotNil(function.GetConfig().Spec.Replicas)
		suite.Require().Equal(zero, *function.GetConfig().Spec.Replicas)

		// function deployment replicas should remain 0
		functionDeployment := suite.GetFunctionDeployment(functionName)
		suite.Require().Equal(int(functionDeployment.Status.Replicas), zero)
		return true
	})
}

func TestFunctionMonitoringTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(FunctionMonitoringTestSuite))
}
