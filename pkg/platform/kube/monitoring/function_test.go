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

package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioiofake "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned/fake"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	autosv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type FunctionMonitoringTestSuite struct {
	suite.Suite
	nuclioioClientSet  *nuclioiofake.Clientset
	kubeClientSet      *fake.Clientset
	Namespace          string
	Logger             logger.Logger
	functionMonitor    *FunctionMonitor
	scalingGracePeriod time.Duration
	ctx                context.Context
}

func (suite *FunctionMonitoringTestSuite) SetupSuite() {
	var err error
	suite.Namespace = "default-namespace"
	suite.ctx = context.Background()
	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Failed to create logger")
}

func (suite *FunctionMonitoringTestSuite) SetupTest() {
	var err error
	suite.kubeClientSet = fake.NewSimpleClientset()
	suite.nuclioioClientSet = nuclioiofake.NewSimpleClientset()
	suite.scalingGracePeriod = 20 * time.Second
	suite.functionMonitor, err = NewFunctionMonitor(suite.ctx,
		suite.Logger,
		suite.Namespace,
		suite.kubeClientSet,
		suite.nuclioioClientSet,
		time.Second,
		suite.scalingGracePeriod,
		2*time.Minute)
	suite.Require().NoError(err)
}

func (suite *FunctionMonitoringTestSuite) TestBulkCheckFunctionStatuses() {

	// create some dummy functions in provisioning mode
	for i := 0; i < 100; i++ {
		function, err := suite.nuclioioClientSet.NuclioV1beta1().
			NuclioFunctions(suite.Namespace).
			Create(suite.ctx,
				&nuclioio.NuclioFunction{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("func-%d", i),
						Namespace: suite.Namespace,
					},
					Spec: functionconfig.Spec{},
					Status: functionconfig.Status{
						State: functionconfig.FunctionStateWaitingForResourceConfiguration,
					},
				}, metav1.CreateOptions{})
		suite.Require().NoError(err)
		suite.Require().NotNil(function)
	}

	err := suite.functionMonitor.checkFunctionStatuses(suite.ctx)
	suite.Require().NoError(err)
}

func (suite *FunctionMonitoringTestSuite) TestStatusNotChangeWhileScaling() {

	minReplicas := 0
	minReplicasInt32 := int32(minReplicas)
	maxReplicas := 4
	maxReplicasInt32 := int32(maxReplicas)

	// create a dummy function in ready state
	functionSpec := functionconfig.Spec{
		MinReplicas: &minReplicas,
		MaxReplicas: &maxReplicas,
	}
	function, err := suite.nuclioioClientSet.NuclioV1beta1().
		NuclioFunctions(suite.Namespace).
		Create(suite.ctx,
			&nuclioio.NuclioFunction{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "func",
					Namespace: suite.Namespace,
				},
				Spec: functionSpec,
				Status: functionconfig.Status{
					State: functionconfig.FunctionStateReady,
				},
			}, metav1.CreateOptions{})
	suite.Require().NoError(err)
	suite.Require().NotNil(function)

	// create a deployment for the function, with status not ready
	deploymentSpec := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kube.DeploymentNameFromFunctionName(function.Name),
			Namespace: suite.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kube.PodNameFromFunctionName(function.Name),
					Namespace: suite.Namespace,
				},
				Spec: corev1.PodSpec{},
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:            4,
			AvailableReplicas:   1,
			UnavailableReplicas: 3,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}

	deployment, err := suite.kubeClientSet.AppsV1().
		Deployments(function.Namespace).
		Create(suite.ctx, &deploymentSpec, metav1.CreateOptions{})
	suite.Require().NoError(err)
	suite.Require().NotNil(deployment)

	// mock an HPA for the function, with current replicas < desired replicas
	now := metav1.Now()
	hpaSpec := autosv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kube.HPANameFromFunctionName(function.Name),
			Namespace: suite.Namespace,
		},
		Spec: autosv2.HorizontalPodAutoscalerSpec{
			MinReplicas: &minReplicasInt32,
			MaxReplicas: maxReplicasInt32,
			ScaleTargetRef: autosv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       kube.DeploymentNameFromFunctionName(function.Name),
			},
		},
		Status: autosv2.HorizontalPodAutoscalerStatus{
			LastScaleTime:   &now,
			CurrentReplicas: 1,
			DesiredReplicas: 4,
		},
	}
	hpa, err := suite.kubeClientSet.AutoscalingV2().
		HorizontalPodAutoscalers(suite.Namespace).
		Create(suite.ctx, &hpaSpec, metav1.CreateOptions{})
	suite.Require().NoError(err)
	suite.Require().NotNil(hpa)

	// invoke the function monitor and make sure the function status is not changed
	err = suite.functionMonitor.checkFunctionStatuses(suite.ctx)
	suite.Require().NoError(err)

	// get the function and make sure its status is still ready
	function, err = suite.nuclioioClientSet.NuclioV1beta1().
		NuclioFunctions(suite.Namespace).
		Get(suite.ctx, function.Name, metav1.GetOptions{})
	suite.Require().NoError(err)
	suite.Require().NotNil(function)
	suite.Require().Equal(functionconfig.FunctionStateReady, function.Status.State)

	// wait for the scaling grace period to pass
	time.Sleep(suite.scalingGracePeriod)

	// invoke the function monitor and make sure the function status is now changed to unhealthy
	err = suite.functionMonitor.checkFunctionStatuses(suite.ctx)
	suite.Require().NoError(err)

	// get the function and make sure its status is now unhealthy
	function, err = suite.nuclioioClientSet.NuclioV1beta1().
		NuclioFunctions(suite.Namespace).
		Get(suite.ctx, function.Name, metav1.GetOptions{})
	suite.Require().NoError(err)
	suite.Require().NotNil(function)
	suite.Require().Equal(functionconfig.FunctionStateUnhealthy, function.Status.State)
}

func TestFunctionMonitoringTestSuite(t *testing.T) {
	suite.Run(t, new(FunctionMonitoringTestSuite))
}
