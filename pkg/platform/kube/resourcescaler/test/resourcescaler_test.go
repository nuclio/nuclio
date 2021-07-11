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
	"net/http"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/resourcescaler"
	"github.com/nuclio/nuclio/pkg/platform/kube/test"

	"github.com/stretchr/testify/suite"
	"github.com/v3io/scaler/pkg/autoscaler"
	"github.com/v3io/scaler/pkg/dlx"
	"github.com/v3io/scaler/pkg/scalertypes"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
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

	suite.KubeTestSuite.SetupSuite()
	resourceScaler, err := resourcescaler.New(suite.Logger,
		suite.Namespace,
		suite.FunctionClientSet,
		suite.PlatformConfiguration)
	suite.Require().NoError(err)
	suite.resourceScaler = resourceScaler.(*resourcescaler.NuclioResourceScaler)

	resourceScalerConfig, err := resourceScaler.GetConfig()
	suite.Require().NoError(err)

	resourceScalerConfig.AutoScalerOptions.ScaleInterval = scalertypes.Duration{
		Duration: 5 * time.Second,
	}

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
		ScaleResources: []functionconfig.ScaleResource{
			{
				MetricName: "something",
				Threshold:  1,
				WindowSize: "250ms",
			},
		},
	}

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// add metrics data
		// make metric client return value that is small enough (e.g.: 0)
		// to ensure scaler will scale the function to zero
		suite.metricClient.AddReactor("get", "nucliofunctions.nuclio.io", func(action k8stesting.Action) (
			handled bool, ret runtime.Object, err error) {
			return true, &v1beta2.MetricValueList{
				Items: []v1beta2.MetricValue{
					{
						DescribedObject: v1.ObjectReference{
							Name:      functionName,
							Namespace: suite.Namespace,
						},
						Value: resource.MustParse("0"),
					},
				},
			}, nil
		})

		// wait for the function to scale to zero
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

		// function has scaled to zero, remove the reactor we added to send "fake" metric values
		suite.metricClient.ReactionChain = suite.metricClient.ReactionChain[:len(suite.metricClient.ReactionChain)-1]

		// try invoke function without the target header
		// expect DLX to fail on 400
		_, _, _ = common.SendHTTPRequest(http.MethodGet,
			fmt.Sprintf("http://%s:8080", suite.GetTestHost()),
			[]byte{},
			map[string]string{},
			nil,
			http.StatusBadRequest,
			true,
			30*time.Second)

		// add target header, expect it to wake up the function
		// for this specific test case, the response status code is 502
		// reason dlx tries to reverse-proxy the request to the function by its service
		// and since the dlx component is running as a process (and not as a POD)
		// it fails to resolve the internal (kubernetes) function host
		// TODO: make DLX work in "test" mode, where it invoke the function from within the k8s cluster
		//       see suite.KubectlInvokeFunctionViaCurl(functionName, "http://function-service-endpoint:8080")
		responseBody, _, err := common.SendHTTPRequest(http.MethodGet,
			fmt.Sprintf("http://%s:8080", suite.GetTestHost()),
			[]byte{},
			map[string]string{
				"X-Nuclio-Target": functionName,
			},
			nil,
			0,
			true,
			30*time.Second)
		suite.Require().NoError(err)
		suite.Require().Equal([]byte{}, responseBody)

		// function has woken up
		suite.WaitForFunctionState(&platform.GetFunctionsOptions{
			Namespace: suite.Namespace,
			Name:      functionName,
		},
			functionconfig.FunctionStateReady,
			30*time.Second)
		return true
	})
}

func TestControllerTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(ResourceScalerTestSuite))
}
