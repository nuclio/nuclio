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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/resourcescaler"
	"github.com/nuclio/nuclio/pkg/platform/kube/test"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/stretchr/testify/suite"
	"github.com/v3io/scaler/pkg/autoscaler"
	"github.com/v3io/scaler/pkg/dlx"
	"github.com/v3io/scaler/pkg/scalertypes"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/metrics/pkg/apis/custom_metrics/v1beta2"
	"k8s.io/metrics/pkg/client/custom_metrics/fake"
)

type ResourceScalerTestSuite struct {
	test.KubeTestSuite
	dlx            *dlx.DLX
	autoscaler     *autoscaler.Autoscaler
	metricClient   *fake.FakeCustomMetricsClient
	resourceScaler *resourcescaler.NuclioResourceScaler
}

func (suite *ResourceScalerTestSuite) SetupSuite() {
	var err error

	suite.PlatformConfiguration = &platformconfig.Config{}
	suite.PlatformConfiguration.ScaleToZero.ScaleResources = []functionconfig.ScaleResource{
		{
			MetricName: "something",
			WindowSize: "1m",
			Threshold:  5,
		},
	}

	suite.KubeTestSuite.SetupSuite()
	resourceScaler, err := resourcescaler.New(suite.Logger,
		suite.Namespace,
		suite.FunctionClientSet,
		suite.PlatformConfiguration)
	suite.Require().NoError(err)
	suite.resourceScaler = resourceScaler.(*resourcescaler.NuclioResourceScaler)

	resourceScalerConfig, err := resourceScaler.GetConfig()
	suite.Require().NoError(err)

	suite.dlx, err = dlx.NewDLX(suite.Logger, resourceScaler, resourceScalerConfig.DLXOptions)
	suite.Require().NoError(err)

	suite.metricClient = &fake.FakeCustomMetricsClient{}
	suite.autoscaler, err = autoscaler.NewAutoScaler(suite.Logger,
		resourceScaler,
		suite.metricClient,
		resourceScalerConfig.AutoScalerOptions)
	suite.Require().NoError(err)

}
func (suite *ResourceScalerTestSuite) SetupTest() {
	suite.KubeTestSuite.SetupTest()
	go func() {
		err := suite.dlx.Start()
		suite.Require().NoError(err, "Failed to start DLX server")
	}()

	go func() {
		err := suite.autoscaler.Start()
		suite.Require().NoError(err, "Failed to start AutoScaler server")
	}()
}

func (suite *ResourceScalerTestSuite) TearDownTest() {
	err := suite.dlx.Stop(context.TODO())
	suite.Require().NoError(err, "Failed to stop DLX server")

	err = suite.autoscaler.Stop()
	suite.Require().NoError(err, "Failed to stop AutoScaler server")
	suite.KubeTestSuite.TearDownTest()
}

// TestSanity scale function to / from zero
func (suite *ResourceScalerTestSuite) TestSanity() {
	functionName := fmt.Sprintf("resourcescaler-test-%s", suite.TestID)
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)
	zero := 0
	one := 1
	createFunctionOptions.FunctionConfig.Spec.MinReplicas = &zero
	createFunctionOptions.FunctionConfig.Spec.MaxReplicas = &one
	createFunctionOptions.FunctionConfig.Spec.ScaleToZero = &functionconfig.ScaleToZeroSpec{
		ScaleResources: suite.PlatformConfiguration.ScaleToZero.ScaleResources,
	}

	// when autoscaler requests metric from the custom metric client
	// ensure that its response is mocked
	suite.mockMetricClient(functionName, resource.MustParse("0"))

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		suite.scaleFunctionToZero(functionName)
		suite.scaleFunctionFromZero(functionName)
		return true
	})
}

func (suite *ResourceScalerTestSuite) scaleFunctionToZero(functionName string) {
	suite.Logger.InfoWith("Scaling function to zero", "functionName", functionName)

	// get function resources
	resources, err := suite.resourceScaler.GetResources()
	suite.Require().NoError(err)

	// scale function to zero
	err = suite.resourceScaler.SetScale(resources, 0)
	suite.Require().NoError(err)

	// wait for function to scale
	suite.WaitForFunctionState(&platform.GetFunctionsOptions{
		Namespace: suite.Namespace,
		Name:      functionName,
	},
		functionconfig.FunctionStateScaledToZero,
		30*time.Second)

	// ensure function deployment replicas is zero
	suite.WaitForFunctionDeployment(functionName, 15*time.Second, func(deployment *appsv1.Deployment) bool {
		return deployment.Status.Replicas == 0
	})
}

func (suite *ResourceScalerTestSuite) scaleFunctionFromZero(functionName string) {
	suite.Logger.InfoWith("Scaling function from zero", "functionName", functionName)

	// scale function from zero
	err := suite.resourceScaler.SetScale([]scalertypes.Resource{
		{
			Name:      functionName,
			Namespace: suite.Namespace,
		},
	}, 1)
	suite.Require().NoError(err)

	// wait for function to scale
	suite.WaitForFunctionState(&platform.GetFunctionsOptions{
		Namespace: suite.Namespace,
		Name:      functionName,
	},
		functionconfig.FunctionStateReady,
		30*time.Second)

	// ensure function deployment replicas is zero
	suite.WaitForFunctionDeployment(functionName, 15*time.Second, func(deployment *appsv1.Deployment) bool {
		return deployment.Status.Replicas == 1
	})
}

func (suite *ResourceScalerTestSuite) mockMetricClient(functionName string, value resource.Quantity) {
	config, _ := suite.resourceScaler.GetConfig()
	windowSizeDuration, _ := time.ParseDuration(suite.PlatformConfiguration.ScaleToZero.ScaleResources[0].WindowSize)

	functionResource := scalertypes.Resource{}
	functionResource.ScaleResources = []scalertypes.ScaleResource{
		{
			MetricName: suite.PlatformConfiguration.ScaleToZero.ScaleResources[0].MetricName,
			WindowSize: scalertypes.Duration{
				Duration: windowSizeDuration,
			},
			Threshold: suite.PlatformConfiguration.ScaleToZero.ScaleResources[0].Threshold,
		},
	}
	action := fake.NewGetForAction(config.AutoScalerOptions.GroupKind,
		suite.Namespace,
		"*",
		functionResource.ScaleResources[0].GetKubernetesMetricName(), labels.Everything())
	_, err := suite.metricClient.Invokes(action, &v1beta2.MetricValueList{
		Items: []v1beta2.MetricValue{
			{
				DescribedObject: v1.ObjectReference{
					Name:      functionName,
					Namespace: suite.Namespace,
				},
				Value: value,
			},
		},
	})
	suite.Require().NoError(err)
}

func TestControllerTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(ResourceScalerTestSuite))
}
