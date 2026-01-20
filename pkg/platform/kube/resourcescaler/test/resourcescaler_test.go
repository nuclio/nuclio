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
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/common/testutils"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/resourcescaler"
	kubesuite "github.com/nuclio/nuclio/pkg/platform/kube/test/suite"
	httptrigger "github.com/nuclio/nuclio/pkg/processor/trigger/http"

	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	"github.com/v3io/scaler/pkg/autoscaler"
	"github.com/v3io/scaler/pkg/dlx"
	"github.com/v3io/scaler/pkg/scalertypes"
	appsv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/kubernetes"
)

// mockMetricClient implements scalertypes.MetricsClient for testing purposes.
// It allows tests to control metric values programmatically, enabling predictable
// scale-to-zero behavior without relying on external metrics systems.
type mockMetricClient struct {
	// scaleToZeroConfig maps function:metric keys to whether the metric should indicate
	// scale-to-zero (true = return 0) or keep-running (false = return threshold+1).
	// The composite key format is "functionName:metricName" to avoid conflicts
	// when multiple functions use the same metric name.
	scaleToZeroConfig map[string]bool
}

// GetResourceMetrics implements scalertypes.MetricsClient interface.
// Returns metric values for the requested resources based on test configuration.
//
// Behavior:
//   - If a metric is configured via setMetricValue, returns the configured value:
//   - shouldScaleToZero=true: returns 0 (below threshold, triggers scale-to-zero)
//   - shouldScaleToZero=false: returns threshold+1 (above threshold, keeps function running)
//   - If a metric is not configured, defaults to 0 (allows scale-to-zero)
func (m *mockMetricClient) GetResourceMetrics(resources []scalertypes.Resource) (map[string]map[string]int, error) {
	result := make(map[string]map[string]int, len(resources))

	for _, r := range resources {
		resourceMetrics := make(map[string]int)

		for _, scaleResource := range r.ScaleResources {
			metricName := scaleResource.GetKubernetesMetricName()
			key := m.buildMetricKey(r.Name, metricName)

			shouldScaleToZero, isConfigured := m.scaleToZeroConfig[key]
			if !isConfigured {
				// Default behavior: return 0 to allow scale-to-zero when not explicitly configured
				resourceMetrics[metricName] = 0
				continue
			}

			if shouldScaleToZero {
				// Metric value below threshold triggers scale-to-zero
				resourceMetrics[metricName] = 0
			} else {
				// Return threshold+1 to ensure value > threshold, preventing scale-to-zero
				// The autoscaler uses "value > threshold" comparison, so threshold+1 keeps the function running
				resourceMetrics[metricName] = scaleResource.Threshold + 1
			}
		}
		result[r.Name] = resourceMetrics
	}
	return result, nil
}

type ResourceScalerTestSuite struct {
	kubesuite.KubeTestSuite
	dlx            *dlx.DLX
	autoscaler     *autoscaler.Autoscaler
	metricClient   *mockMetricClient
	resourceScaler *resourcescaler.NuclioResourceScaler
	dlxHTTPClient  *http.Client
}

func (suite *ResourceScalerTestSuite) SetupSuite() {
	var err error

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

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

	// create kube client set
	restConfig, err := common.GetClientConfig(common.GetKubeconfigPath(""))
	suite.Require().NoError(err)

	resourceScalerConfig.DLXOptions.KubeClientSet, err = kubernetes.NewForConfig(restConfig)
	suite.Require().NoError(err)

	suite.dlx, err = dlx.NewDLX(suite.Logger, resourceScaler, resourceScalerConfig.DLXOptions)
	suite.Require().NoError(err)

	go func() {
		err := suite.dlx.Start()
		suite.Require().NoError(err, "Failed to start DLX server")
	}()

	suite.metricClient = newMockMetricClient()
	suite.autoscaler, err = autoscaler.NewAutoScaler(suite.Logger,
		resourceScaler,
		suite.metricClient,
		resourceScalerConfig.AutoScalerOptions)
	suite.Require().NoError(err)

}

func (suite *ResourceScalerTestSuite) SetupTest() {
	suite.KubeTestSuite.SetupTest()
	suite.metricClient.scaleToZeroConfig = make(map[string]bool)

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

		// set metric value to 0 (scale to zero) using the mock metric client
		suite.metricClient.setMetricValue(functionName, "something_per_250ms", true)

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

		// set metric value to 1 to keep function up after wake-up
		// This must be done BEFORE the HTTP request so that when DLX wakes it up,
		// the autoscaler will see non-zero metrics and keep it up
		suite.metricClient.setMetricValue(functionName, "something_per_250ms", false)

		// try invoke function without the target header
		// expect DLX to fail on 400
		_, _, _ = common.SendHTTPRequest(suite.dlxHTTPClient,
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
		responseBody, _, err := common.SendHTTPRequest(suite.dlxHTTPClient,
			http.MethodGet,
			fmt.Sprintf("http://%s:8080", suite.GetTestHost()),
			[]byte{},
			map[string]string{
				headers.TargetName: functionName,
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

		// set metric value to 0 (scale to zero) using the mock metric client
		suite.metricClient.setMetricValue(functionName, "something_per_250ms", true)

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
	}

	suite.DeployFunction(createFunctionOptions1, func(deployResult *platform.CreateFunctionResult) bool {
		suite.DeployFunction(createFunctionOptions2, func(deployResult *platform.CreateFunctionResult) bool {

			apiGatewayName := "api-gateway-test"
			createAPIGatewayOptions := suite.CompileCreateAPIGatewayOptions(apiGatewayName, functionName1, functionName2)
			err := suite.DeployAPIGateway(createAPIGatewayOptions, func(*networkingv1.Ingress) {
				scaleToZero(functionName1)
				scaleToZero(functionName2)

				// set metric values to keep functions up after wake-up
				// This must be done BEFORE the HTTP request so that when DLX wakes them up,
				// the autoscaler will see non-zero metrics and keep them up
				suite.metricClient.setMetricValue(functionName1, "something_per_250ms", false)
				suite.metricClient.setMetricValue(functionName2, "something_per_250ms", false)

				// add target header, expect it to wake up both functions
				// for this specific test case, the response status code is 502
				// reason dlx tries to reverse-proxy the request to the function by its service
				// and since the dlx component is running as a process (and not as a POD)
				// it fails to resolve the internal (kubernetes) function host
				// Background: make DLX work in "test" mode, where it invoke the function from within the k8s cluster
				//       see suite.KubectlInvokeFunctionViaCurl(functionName, "http://function-service-endpoint:8080")
				responseBody, _, err := common.SendHTTPRequest(suite.dlxHTTPClient,
					http.MethodGet,
					fmt.Sprintf("http://%s:8080", suite.GetTestHost()),
					[]byte{},
					map[string]string{
						headers.TargetName: fmt.Sprintf("%s,%s", functionName1, functionName2),
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

// newMockMetricClient creates a new mock metrics client instance.
func newMockMetricClient() *mockMetricClient {
	return &mockMetricClient{
		scaleToZeroConfig: make(map[string]bool),
	}
}

// buildMetricKey creates a composite key from function and metric names.
// This ensures each function's metrics are tracked independently, even when
// multiple functions share the same metric name.
func (m *mockMetricClient) buildMetricKey(functionName, metricName string) string {
	return functionName + ":" + metricName
}

// setMetricValue configures the metric value for a specific function and metric to control scale-to-zero behavior.
func (m *mockMetricClient) setMetricValue(functionName, metricName string, shouldScaleToZero bool) {
	key := m.buildMetricKey(functionName, metricName)
	m.scaleToZeroConfig[key] = shouldScaleToZero
}
