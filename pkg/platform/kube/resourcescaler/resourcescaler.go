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

package resourcescaler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/headers"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	httptrigger "github.com/nuclio/nuclio/pkg/processor/trigger/http"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/v3io/scaler/pkg/scalertypes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NuclioResourceScaler leverages github.com/v3io/scaler
// to allow extending scale to zero and from zero nuclio functions
type NuclioResourceScaler struct {
	logger                               logger.Logger
	nuclioClientSet                      nuclioioclient.Interface
	namespace                            string
	platformConfiguration                *platformconfig.Config
	httpClient                           *http.Client
	functionReadinessVerificationEnabled bool
}

func New(logger logger.Logger,
	namespace string,
	nuclioClientSet nuclioioclient.Interface,
	platformConfiguration *platformconfig.Config) (scalertypes.ResourceScaler, error) {

	logger.DebugWith("Initializing resource scaler",
		"platformconfig", platformConfiguration)

	return &NuclioResourceScaler{
		logger:                logger,
		namespace:             namespace,
		nuclioClientSet:       nuclioClientSet,
		platformConfiguration: platformConfiguration,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}, nil
}

func (n *NuclioResourceScaler) SetScale(resources []scalertypes.Resource, scale int) error {
	return n.SetScaleCtx(context.Background(), resources, scale)
}

func (n *NuclioResourceScaler) SetScaleCtx(ctx context.Context, resources []scalertypes.Resource, scale int) error {
	if scale == 0 {
		return n.scaleFunctionsToZero(ctx, resources)
	}
	return n.scaleFunctionsFromZero(ctx, resources)
}

// SetHTTPClient sets the http client for testing purposes
func (n *NuclioResourceScaler) SetHTTPClient(httpClient *http.Client) {
	n.httpClient = httpClient
}

// GetHTTPClient returns the http client for testing purposes
func (n *NuclioResourceScaler) GetHTTPClient() *http.Client {
	return n.httpClient
}

func (n *NuclioResourceScaler) SetFunctionReadinessVerificationEnabled(enable bool) {
	n.logger.InfoWith("Setting function readiness verification", "enable", enable)
	n.functionReadinessVerificationEnabled = enable
}

func (n *NuclioResourceScaler) GetResources() ([]scalertypes.Resource, error) {
	var functionList []scalertypes.Resource
	functions, err := n.nuclioClientSet.
		NuclioV1beta1().
		NuclioFunctions(n.namespace).
		List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list functions")
	}

	// build a list of function names that are candidates to be scaled to zero
	for _, function := range functions.Items {

		// don't include functions that aren't in ready state or that min replicas is larger than zero
		if function.GetComputedMinReplicas() <= 0 && function.Status.State == functionconfig.FunctionStateReady {
			if function.Spec.ScaleToZero == nil {
				n.logger.WarnWith("Function missing scale to zero spec. Continuing", "functionName", function.Name)
				continue
			}

			scaleResources, err := n.parseScaleResources(function)
			if err != nil {
				n.logger.WarnWith("Failed to parse scale resources. Continuing", "functionName", function.Name)
				continue
			}

			lastScaleEvent, lastScaleEventTime, err := n.parseLastScaleEvent(function)
			if err != nil {
				n.logger.WarnWith("Failed to parse last scale event. Continuing", "functionName", function.Name)
				continue
			}

			functionList = append(functionList, scalertypes.Resource{
				Name:               function.Name,
				Namespace:          function.Namespace,
				ScaleResources:     scaleResources,
				LastScaleEvent:     lastScaleEvent,
				LastScaleEventTime: lastScaleEventTime,
			})
		}
	}
	return functionList, nil
}

func (n *NuclioResourceScaler) GetConfig() (*scalertypes.ResourceScalerConfig, error) {

	// enrich
	if n.platformConfiguration.ScaleToZero.ResourceReadinessTimeout == "" {
		n.platformConfiguration.ScaleToZero.ResourceReadinessTimeout = "2m"
	}

	if n.platformConfiguration.ScaleToZero.ScalerInterval == "" {
		n.platformConfiguration.ScaleToZero.ScalerInterval = "1m"
	}

	scaleInterval, err := time.ParseDuration(n.platformConfiguration.ScaleToZero.ScalerInterval)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse scaler interval duration")
	}

	resourceReadinessTimeout, err := time.ParseDuration(n.platformConfiguration.ScaleToZero.ResourceReadinessTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse resource readiness timeout")
	}

	return &scalertypes.ResourceScalerConfig{
		AutoScalerOptions: scalertypes.AutoScalerOptions{
			Namespace:     n.namespace,
			ScaleInterval: scalertypes.Duration{Duration: scaleInterval},
			GroupKind: schema.GroupKind{
				Group: "nuclio.io",
				Kind:  "NuclioFunction",
			},
		},
		DLXOptions: scalertypes.DLXOptions{
			Namespace:                n.namespace,
			TargetPort:               8080,
			TargetNameHeader:         headers.TargetName,
			TargetPathHeader:         "X-Nuclio-Function-Path",
			ListenAddress:            ":8080",
			ResourceReadinessTimeout: scalertypes.Duration{Duration: resourceReadinessTimeout},
			MultiTargetStrategy:      n.platformConfiguration.ScaleToZero.MultiTargetStrategy,
		},
	}, nil
}

func (n *NuclioResourceScaler) ResolveServiceName(resource scalertypes.Resource) (string, error) {
	return kube.ServiceNameFromFunctionName(resource.Name), nil
}

func (n *NuclioResourceScaler) parseScaleResources(function nuclioio.NuclioFunction) ([]scalertypes.ScaleResource, error) {
	var scaleResources []scalertypes.ScaleResource
	for _, scaleResource := range function.Spec.ScaleToZero.ScaleResources {
		windowSize, err := time.ParseDuration(scaleResource.WindowSize)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse window size")
		}
		scaleResources = append(scaleResources, scalertypes.ScaleResource{
			MetricName: scaleResource.MetricName,
			Threshold:  scaleResource.Threshold,
			WindowSize: scalertypes.Duration{Duration: windowSize},
		})
	}
	return scaleResources, nil
}

func (n *NuclioResourceScaler) parseLastScaleEvent(
	function nuclioio.NuclioFunction) (*scalertypes.ScaleEvent, *time.Time, error) {
	if function.Status.ScaleToZero == nil {
		return nil, nil, nil
	}

	if function.Status.ScaleToZero.LastScaleEventTime == nil {
		return nil, nil, errors.New("Function scale to zero status does not contain last scale event time")
	}

	return &function.Status.ScaleToZero.LastScaleEvent, function.Status.ScaleToZero.LastScaleEventTime, nil
}

func (n *NuclioResourceScaler) scaleFunctionsToZero(ctx context.Context, resources []scalertypes.Resource) error {
	failedFunctionNames := make([]string, 0)
	for _, resource := range resources {
		n.logger.DebugWithCtx(ctx,
			"Scaling to zero",
			"functionName", resource.Name,
			"functionNamespace", resource.Namespace)
		if err := n.updateFunctionStatus(ctx,
			resource.Namespace,
			resource.Name,
			functionconfig.FunctionStateWaitingForScaleResourcesToZero,
			scalertypes.ScaleToZeroStartedScaleEvent); err != nil {
			failedFunctionNames = append(failedFunctionNames, resource.Name)
			n.logger.WarnWith("Failed to update function status to scale to zero",
				"functionName", resource.Name,
				"functionNamespace", resource.Namespace)
			continue
		}
	}

	if len(failedFunctionNames) > 0 {
		return errors.Errorf("Failed to scale some functions to zero: %v", failedFunctionNames)
	}
	return nil
}

func (n *NuclioResourceScaler) scaleFunctionsFromZero(ctx context.Context, resources []scalertypes.Resource) error {
	failedFunctionNames := make([]string, 0)
	for _, resource := range resources {
		n.logger.DebugWithCtx(ctx,
			"Scaling from zero",
			"functionName", resource.Name,
			"functionNamespace", resource.Namespace)
		if err := n.updateFunctionStatus(ctx,
			resource.Namespace,
			resource.Name,
			functionconfig.FunctionStateWaitingForScaleResourcesFromZero,
			scalertypes.ScaleFromZeroStartedScaleEvent); err != nil {
			failedFunctionNames = append(failedFunctionNames, resource.Name)
			n.logger.WarnWithCtx(ctx,
				"Failed to update function status to scale from zero",
				"functionName", resource.Name,
				"functionNamespace", resource.Namespace)
			continue
		}
		if err := n.waitFunctionReadiness(ctx, resource.Namespace, resource.Name); err != nil {
			failedFunctionNames = append(failedFunctionNames, resource.Name)
			n.logger.WarnWithCtx(ctx,
				"Failed waiting for function readiness",
				"functionName", resource.Name,
				"functionNamespace", resource.Namespace)
			continue
		}
	}
	if len(failedFunctionNames) > 0 {
		return errors.Errorf("Failed to scale some functions from zero: %v", failedFunctionNames)
	}
	return nil
}

func (n *NuclioResourceScaler) updateFunctionStatus(ctx context.Context,
	namespace string,
	functionName string,
	functionState functionconfig.FunctionState,
	functionScaleEvent scalertypes.ScaleEvent) error {
	function, err := n.nuclioClientSet.
		NuclioV1beta1().
		NuclioFunctions(namespace).
		Get(ctx, functionName, metav1.GetOptions{})
	if err != nil {
		n.logger.WarnWithCtx(ctx,
			"Failed getting nuclio function to update function status",
			"functionName", functionName,
			"err", err)
		return errors.Wrap(err, "Failed getting nuclio function to update function status")
	}

	now := time.Now()
	function.Status.State = functionState
	function.Status.ScaleToZero = &functionconfig.ScaleToZeroStatus{
		LastScaleEvent:     functionScaleEvent,
		LastScaleEventTime: &now,
	}
	if _, err := n.nuclioClientSet.
		NuclioV1beta1().
		NuclioFunctions(namespace).
		Update(ctx,
			function,
			metav1.UpdateOptions{}); err != nil {
		n.logger.WarnWithCtx(ctx,
			"Failed to update function",
			"functionName", functionName,
			"err", err)
		return errors.Wrap(err, "Failed to update nuclio function")
	}
	return nil
}

func (n *NuclioResourceScaler) waitFunctionReadiness(ctx context.Context, namespace string, functionName string) error {
	n.logger.DebugWithCtx(ctx,
		"Waiting for function readiness",
		"functionName", functionName)
	var function *nuclioio.NuclioFunction
	var err error
	for {
		function, err = n.nuclioClientSet.
			NuclioV1beta1().
			NuclioFunctions(namespace).
			Get(ctx, functionName, metav1.GetOptions{})
		if err != nil {
			n.logger.WarnWithCtx(ctx, "Failed getting nuclio function", "functionName", functionName, "err", err)
			return errors.Wrap(err, "Failed getting nuclio function")
		}
		if function.Status.State == functionconfig.FunctionStateReady {
			n.logger.InfoWithCtx(ctx, "Function is ready", "functionName", functionName)
			break
		}

		if ctx.Err() != nil {
			return errors.Wrap(ctx.Err(), "Context error")
		}

		n.logger.DebugWithCtx(ctx,
			"Function is not ready yet",
			"functionName", functionName,
			"currentState", function.Status.State)

		time.Sleep(3 * time.Second)
	}
	return n.verifyReadiness(ctx, function)
}

func (n *NuclioResourceScaler) verifyReadiness(ctx context.Context, function *nuclioio.NuclioFunction) error {
	if !n.functionReadinessVerificationEnabled {
		n.logger.DebugWithCtx(ctx, "Skipping function readiness verification")
	}

	url := fmt.Sprintf("http://%s.%s.svc.cluster.local:8080%s",
		kube.ServiceNameFromFunctionName(function.Name),
		function.Namespace,
		httptrigger.InternalHealthPath)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to create request")
	}

	startTime := time.Now()
	if err := common.RetryUntilSuccessful(time.Minute,
		time.Second*1,
		func() bool {
			response, err := n.httpClient.Do(request)
			if err != nil {
				n.logger.WarnWithCtx(ctx,
					"Failed to send request",
					"err", err,
					"elapsed", time.Since(startTime),
					"url", url)
				return false
			}

			// response is within [200, 300)
			if response.StatusCode >= http.StatusOK && response.StatusCode < 300 {
				n.logger.InfoWithCtx(ctx,
					"Function readiness is verified",
					"elapsed", time.Since(startTime),
					"url", url)
				return true
			}

			n.logger.DebugWithCtx(ctx,
				"Function readiness verification hit unexpected status code, retrying",
				"err", err,
				"elapsed", time.Since(startTime),
				"url", url,
				"statusCode", response.StatusCode)
			return false
		}); err != nil {
		return errors.Wrap(err, "Exhausted waiting for function readiness verification")
	}
	return nil
}
