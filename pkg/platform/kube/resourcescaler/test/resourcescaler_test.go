//go:build test_integration && test_kube

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
	"github.com/nuclio/nuclio/pkg/common/testutils"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/resourcescaler"
	"github.com/nuclio/nuclio/pkg/platform/kube/test"
	httptrigger "github.com/nuclio/nuclio/pkg/processor/trigger/http"

	"github.com/stretchr/testify/suite"
	"github.com/v3io/scaler/pkg/autoscaler"
	"github.com/v3io/scaler/pkg/dlx"
	"github.com/v3io/scaler/pkg/scalertypes"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
	dlxHTTPClient  *http.Client
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

	go func() {
		err := suite.dlx.Start()
		suite.Require().NoError(err, "Failed to start DLX server")
	}()

	suite.metricClient = &fake.FakeCustomMetricsClient{}
	suite.autoscaler, err = autoscaler.NewAutoScaler(suite.Logger,
		resourceScaler,
		suite.metricClient,
		resourceScalerConfig.AutoScalerOptions)
	suite.Require().NoError(err)

}

func (suite *ResourceScalerTestSuite) SetupTest() {
	suite.KubeTestSuite.SetupTest()

	// preserve it, it might be mutated during tests
	suite.dlxHTTPClient = suite.resourceScaler.GetHTTPClient()

	go func() {
		err := suite.autoscaler.Start()
		suite.Require().NoError(err, "Failed to start AutoScaler server")
	}()
}

func (suite *ResourceScalerTestSuite) TearDownTest() {

	// restore
	suite.resourceScaler.SetHTTPClient(suite.dlxHTTPClient)

	// stop auto scaler
	err := suite.autoscaler.Stop()
	suite.Require().NoError(err, "Failed to stop AutoScaler server")
	suite.KubeTestSuite.TearDownTest()
}

func (suite *ResourceScalerTestSuite) TearDownSuite() {
	err := suite.dlx.Stop(context.Background())
	suite.Require().NoError(err, "Failed to stop DLX server")
}

// TestSanity scale function to / from zero
func (suite *ResourceScalerTestSuite) TestSanity() {
	suite.resourceScaler.SetHTTPClient(testutils.CreateDummyHTTPClient(func() func(r *http.Request) *http.Response {
		retryCounter := 0
		return func(request *http.Request) *http.Response {
			if request.URL.Path == httptrigger.InternalHealthPath {
				statusCode := http.StatusBadGateway
				if retryCounter == 3 {
					statusCode = http.StatusOK
				}
				retryCounter += 1
				return &http.Response{
					StatusCode: statusCode,
				}
			}

			suite.Logger.ErrorWith("Unexpected HTTP request was made by resource scaler",
				"request", request)
			panic("Unexpected http request")
		}
	}()))

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
		_, _, _ = common.SendHTTPRequest(nil,
			http.MethodGet,
			fmt.Sprintf("http://%s:8080", suite.GetTestHost()),
			[]byte{},
			map[string]string{},
			nil,
			http.StatusBadRequest)

		// add target header, expect it to wake up the function
		// for this specific test case, the response status code is 502
		// reason dlx tries to reverse-proxy the request to the function by its service
		// and since the dlx component is running as a process (and not as a POD)
		// it fails to resolve the internal (kubernetes) function host
		// Background: make DLX work in "test" mode, where it invoke the function from within the k8s cluster
		//       see suite.KubectlInvokeFunctionViaCurl(functionName, "http://function-service-endpoint:8080")
		responseBody, _, err := common.SendHTTPRequest(nil,
			http.MethodGet,
			fmt.Sprintf("http://%s:8080", suite.GetTestHost()),
			[]byte{},
			map[string]string{
				"X-Nuclio-Target": functionName,
			},
			nil,
			0)
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

func (suite *ResourceScalerTestSuite) TestMultiTargetScaleFromZero() {
	suite.resourceScaler.SetHTTPClient(testutils.CreateDummyHTTPClient(func(request *http.Request) *http.Response {
		if request.URL.Path == httptrigger.InternalHealthPath {
			return &http.Response{
				StatusCode: http.StatusOK,
			}
		}

		suite.Logger.ErrorWith("Unexpected HTTP request was made by resource scaler",
			"request", request)
		panic("Unexpected http request")
	}))
	zero := 0
	one := 1
	functionName1 := fmt.Sprintf("resourcescaler-multi-target-test-1-%s", suite.TestID)
	functionName2 := fmt.Sprintf("resourcescaler-multi-target-test-2-%s", suite.TestID)
	createFunctionOptions1 := suite.CompileCreateFunctionOptions(functionName1)
	createFunctionOptions2 := suite.CompileCreateFunctionOptions(functionName2)
	scalToZeroSpec := functionconfig.ScaleToZeroSpec{
		ScaleResources: []functionconfig.ScaleResource{
			{
				MetricName: "something",
				Threshold:  1,
				WindowSize: "250ms",
			},
		},
	}
	createFunctionOptions1.FunctionConfig.Spec.MinReplicas = &zero
	createFunctionOptions1.FunctionConfig.Spec.MaxReplicas = &one
	createFunctionOptions1.FunctionConfig.Spec.ScaleToZero = &scalToZeroSpec
	createFunctionOptions2.FunctionConfig.Spec.MinReplicas = &zero
	createFunctionOptions2.FunctionConfig.Spec.MaxReplicas = &one
	createFunctionOptions2.FunctionConfig.Spec.ScaleToZero = &scalToZeroSpec

	scaleToZero := func(functionName string) {

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
	}

	suite.DeployFunction(createFunctionOptions1, func(deployResult *platform.CreateFunctionResult) bool {
		suite.DeployFunction(createFunctionOptions2, func(deployResult *platform.CreateFunctionResult) bool {

			apiGatewayName := "api-gateway-test"
			createAPIGatewayOptions := suite.CompileCreateAPIGatewayOptions(apiGatewayName, functionName1, functionName2)
			err := suite.DeployAPIGateway(createAPIGatewayOptions, func(*networkingv1.Ingress) {
				scaleToZero(functionName1)
				scaleToZero(functionName2)

				// add target header, expect it to wake up both functions
				// for this specific test case, the response status code is 502
				// reason dlx tries to reverse-proxy the request to the function by its service
				// and since the dlx component is running as a process (and not as a POD)
				// it fails to resolve the internal (kubernetes) function host
				// Background: make DLX work in "test" mode, where it invoke the function from within the k8s cluster
				//       see suite.KubectlInvokeFunctionViaCurl(functionName, "http://function-service-endpoint:8080")
				responseBody, _, err := common.SendHTTPRequest(nil,
					http.MethodGet,
					fmt.Sprintf("http://%s:8080", suite.GetTestHost()),
					[]byte{},
					map[string]string{
						"X-Nuclio-Target": fmt.Sprintf("%s,%s", functionName1, functionName2),
					},
					nil,
					0)
				suite.Require().NoError(err)
				suite.Require().Equal([]byte{}, responseBody)

				// function has woken up
				suite.WaitForFunctionState(&platform.GetFunctionsOptions{
					Namespace: suite.Namespace,
					Name:      functionName1,
				},
					functionconfig.FunctionStateReady,
					30*time.Second)
				suite.WaitForFunctionState(&platform.GetFunctionsOptions{
					Namespace: suite.Namespace,
					Name:      functionName2,
				},
					functionconfig.FunctionStateReady,
					30*time.Second)
			})
			suite.Require().NoError(err)
			return true
		})
		return true
	})
}

func TestResourceScalerTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(ResourceScalerTestSuite))
}
