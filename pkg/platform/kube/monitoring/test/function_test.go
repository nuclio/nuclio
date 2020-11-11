package test

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/monitoring"
	kubetest "github.com/nuclio/nuclio/pkg/platform/kube/test"

	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionMonitoringTestSuite struct {
	kubetest.KubeTestSuite
}

func (suite *FunctionMonitoringTestSuite) SetupSuite() {
	suite.KubeTestSuite.SetupSuite()

	// decrease blocking interval, to make test run faster
	monitoring.PostDeploymentMonitoringBlockingInterval = time.Second
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

func (suite *FunctionMonitoringTestSuite) TestNoRecoveryAfterDeployError() {
	functionName := "function-deploy-fail"
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

	suite.DeployFunctionExpectErrorAndRedeploy(createFunctionOptions,
		func(deployResult *platform.CreateFunctionResult) bool {
			var err error

			// wait for monitoring
			time.Sleep(postDeploymentSleepInterval)

			// ensure function is still in error state (due to deploy error of missing configmap)
			suite.GetFunctionAndExpectState(getFunctionOptions, functionconfig.FunctionStateError)

			// create the missing configmap
			configMap, err = suite.KubeClientSet.CoreV1().ConfigMaps(suite.Namespace).Create(configMap)
			suite.Require().NoError(err, "Failed to create configmap")

			// wait for k8s to recover deployment from missing configmap error
			suite.WaitForFunctionDeploymentAvailability(functionName, 3*time.Minute)

			// wait for monitoring
			time.Sleep(postDeploymentSleepInterval)

			// ensure function monitoring did not recover the function from its recent deploy error
			suite.GetFunctionAndExpectState(getFunctionOptions, functionconfig.FunctionStateError)

			return true
		}, func(deployResult *platform.CreateFunctionResult) bool {

			// let interval occur at least once
			time.Sleep(postDeploymentSleepInterval)

			// ensure function in ready state, deploy passes
			suite.GetFunctionAndExpectState(getFunctionOptions, functionconfig.FunctionStateReady)
			return true
		})
}

func (suite *FunctionMonitoringTestSuite) TestRecoverErrorStateFunctionWhenResourcesAvailable() {
	functionName := "function-recovery"
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	getFunctionOptions := &platform.GetFunctionsOptions{
		Name:      createFunctionOptions.FunctionConfig.Meta.Name,
		Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
	}

	functionMonitoringSleepTimeout := 2 * suite.Controller.GetFunctionMonitoringInterval()
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
		err = common.RetryUntilSuccessful(functionMonitoringSleepTimeout,
			1*time.Second,
			func() bool {
				pod = suite.GetFunctionPods(functionName)[0]
				suite.Logger.InfoWith("Waiting for function pod",
					"podName", pod.Name,
					"currentPodPhase", pod.Status.Phase,
					"expectedPodPhase", v1.PodRunning)
				return pod.Status.Phase == v1.PodRunning
			})
		suite.Require().NoError(err, "Failed to ensure function pod is running again")

		// wait for function state to become ready again
		suite.WaitForFunctionState(getFunctionOptions,
			functionconfig.FunctionStateReady,
			functionMonitoringSleepTimeout)
		return true
	})
}

func TestFunctionMonitoringTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(FunctionMonitoringTestSuite))
}
