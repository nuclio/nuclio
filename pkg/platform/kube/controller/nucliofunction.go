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

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/client"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"
	"github.com/nuclio/nuclio/pkg/platform/kube/operator"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/v3io/scaler/pkg/scalertypes"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type functionOperator struct {
	logger            logger.Logger
	controller        *Controller
	operator          operator.Operator
	imagePullSecrets  string
	functionresClient functionres.Client
}

func newFunctionOperator(parentLogger logger.Logger,
	controller *Controller,
	resyncInterval *time.Duration,
	imagePullSecrets string,
	functionresClient functionres.Client,
	numWorkers int) (*functionOperator, error) {
	var err error

	loggerInstance := parentLogger.GetChild("function")

	newFunctionOperator := &functionOperator{
		logger:            loggerInstance,
		controller:        controller,
		imagePullSecrets:  imagePullSecrets,
		functionresClient: functionresClient,
	}

	// create a function operator
	newFunctionOperator.operator, err = operator.NewMultiWorker(loggerInstance,
		numWorkers,
		newFunctionOperator.getListWatcher(controller.namespace),
		&nuclioio.NuclioFunction{},
		resyncInterval,
		newFunctionOperator)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function operator")
	}

	parentLogger.DebugWith("Created function operator",
		"numWorkers", numWorkers,
		"resyncInterval", resyncInterval)

	return newFunctionOperator, nil
}

// CreateOrUpdate handles creation/update of an object
func (fo *functionOperator) CreateOrUpdate(ctx context.Context, object runtime.Object) error {
	function, objectIsFunction := object.(*nuclioio.NuclioFunction)
	if !objectIsFunction {
		return fo.setFunctionError(nil,
			functionconfig.FunctionStateError,
			errors.New("Received unexpected object, expected function"))
	}

	defer common.CatchAndLogPanicWithOptions(ctx, // nolint: errcheck
		fo.logger,
		"nucliofunction.CreateOrUpdate",
		&common.CatchAndLogPanicOptions{
			Args: []interface{}{
				"function", function,
			},
			CustomHandler: func(panicError error) {
				fo.setFunctionError(function, // nolint: errcheck
					functionconfig.FunctionStateError,
					errors.Wrap(panicError, "Failed to create/update function"))
			},
		})

	// validate function name is according to k8s convention
	errorMessages := validation.IsQualifiedName(function.Name)
	if len(errorMessages) != 0 {
		joinedErrorMessage := strings.Join(errorMessages, ", ")
		return errors.New("Function name doesn't conform to k8s naming convention. Errors: " + joinedErrorMessage)
	}

	// ready functions as part of controller resyncs, where we verify that a given function CRD has its resources
	// properly configured
	statesToRespond := []functionconfig.FunctionState{

		// monitor provisioning states, we need to create / update function resources
		functionconfig.FunctionStateWaitingForResourceConfiguration,
		functionconfig.FunctionStateWaitingForScaleResourcesFromZero,
		functionconfig.FunctionStateWaitingForScaleResourcesToZero,

		// to know when to scale a function to zero
		functionconfig.FunctionStateReady,

		// to know when to scale a function from zero
		functionconfig.FunctionStateScaledToZero,
	}
	if !functionconfig.FunctionStateInSlice(function.Status.State, statesToRespond) {
		fo.logger.DebugWith("NuclioFunction is not waiting for resource creation or ready, skipping create/update",
			"name", function.Name,
			"state", function.Status.State,
			"namespace", function.Namespace)

		return nil
	}

	// imported functions have skip deploy annotation, set its state and bail
	if functionconfig.ShouldSkipDeploy(function.Annotations) {
		fo.logger.InfoWith("Skipping function deploy",
			"name", function.Name,
			"state", function.Status.State,
			"namespace", function.Namespace)
		return fo.setFunctionStatus(function, &functionconfig.Status{
			State: functionconfig.FunctionStateImported,
		})
	}

	// wait for up to the default readiness timeout or whatever was set in the spec
	readinessTimeout := function.Spec.ReadinessTimeoutSeconds
	if readinessTimeout == 0 {
		readinessTimeout = int(fo.
			controller.
			GetPlatformConfiguration().
			GetDefaultFunctionReadinessTimeout().Seconds())
	}

	fo.logger.DebugWith("Ensuring function resources",
		"functionNamespace", function.Namespace,
		"readinessTimeout", readinessTimeout,
		"functionName", function.Name)

	// ensure function resources (deployment, ingress, configmap, etc ...)
	resources, err := fo.functionresClient.CreateOrUpdate(ctx, function, fo.imagePullSecrets)
	if err != nil {
		return fo.setFunctionError(function,
			functionconfig.FunctionStateError,
			errors.Wrap(err, "Failed to create/update function"))
	}

	// readinessTimeout would be zero when
	// - not defined on function spec
	// - defined 0 on platform-config
	if readinessTimeout != 0 {
		waitContext, cancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(readinessTimeout)*time.Second))
		defer cancel()

		// wait until the function resources are ready
		if err = fo.functionresClient.WaitAvailable(waitContext, function.Namespace, function.Name); err != nil {
			return fo.setFunctionError(function,
				functionconfig.FunctionStateUnhealthy,
				errors.Wrap(err, "Failed to wait for function resources to be available"))
		}
	}

	waitingStates := []functionconfig.FunctionState{
		functionconfig.FunctionStateWaitingForResourceConfiguration,
		functionconfig.FunctionStateWaitingForScaleResourcesFromZero,
		functionconfig.FunctionStateWaitingForScaleResourcesToZero,
	}

	if functionconfig.FunctionStateInSlice(function.Status.State, waitingStates) {

		var scaleEvent scalertypes.ScaleEvent
		var finalState functionconfig.FunctionState
		switch function.Status.State {
		case functionconfig.FunctionStateWaitingForScaleResourcesToZero:
			scaleEvent = scalertypes.ScaleToZeroCompletedScaleEvent
			finalState = functionconfig.FunctionStateScaledToZero
		case functionconfig.FunctionStateWaitingForScaleResourcesFromZero:
			scaleEvent = scalertypes.ScaleFromZeroCompletedScaleEvent
			finalState = functionconfig.FunctionStateReady
		case functionconfig.FunctionStateWaitingForResourceConfiguration:
			scaleEvent = scalertypes.ResourceUpdatedScaleEvent
			finalState = functionconfig.FunctionStateReady
		}

		// NOTE: this reconstructs function status and hence omits all other function status fields
		// ... such as message and logs.
		functionStatus := &functionconfig.Status{
			State: finalState,
		}

		if err := fo.populateFunctionInvocationStatus(function, functionStatus, resources); err != nil {
			return errors.Wrap(err, "Failed to populate function invocation status")
		}

		if err := fo.setFunctionScaleToZeroStatus(ctx, functionStatus, scaleEvent); err != nil {
			return errors.Wrap(err, "Failed setting function scale to zero status")
		}

		return fo.setFunctionStatus(function, functionStatus)
	}

	return nil
}

// Delete handles delete of an object
func (fo *functionOperator) Delete(ctx context.Context, namespace string, name string) error {
	fo.logger.DebugWith("Deleting function",
		"name", name,
		"namespace", namespace)

	return fo.functionresClient.Delete(ctx, namespace, name)
}

func (fo *functionOperator) setFunctionScaleToZeroStatus(ctx context.Context,
	functionStatus *functionconfig.Status,
	scaleToZeroEvent scalertypes.ScaleEvent) error {

	fo.logger.DebugWith("Setting scale to zero status",
		"LastScaleEvent", scaleToZeroEvent)
	now := time.Now()
	functionStatus.ScaleToZero = &functionconfig.ScaleToZeroStatus{
		LastScaleEvent:     scaleToZeroEvent,
		LastScaleEventTime: &now,
	}
	return nil
}

func (fo *functionOperator) start() error {
	go fo.operator.Start() // nolint: errcheck

	return nil
}

func (fo *functionOperator) setFunctionError(function *nuclioio.NuclioFunction,
	functionErrorState functionconfig.FunctionState,
	err error) error {

	// whatever the error, try to update the function CR
	fo.logger.WarnWith("Setting function error",
		"functionErrorState", functionErrorState,
		"functionName", function.Name,
		"err", err)

	if setStatusErr := fo.setFunctionStatus(function, &functionconfig.Status{
		State:   functionErrorState,
		Message: errors.GetErrorStackString(err, 10),
	}); setStatusErr != nil {
		fo.logger.WarnWith("Failed to update function on error",
			"setStatusErr", errors.Cause(setStatusErr))
	}

	return err
}

func (fo *functionOperator) setFunctionStatus(function *nuclioio.NuclioFunction,
	status *functionconfig.Status) error {

	fo.logger.DebugWith("Setting function state", "name", function.Name, "status", status)

	// indicate error state
	function.Status = *status

	// try to update the function
	_, err := fo.controller.nuclioClientSet.NuclioV1beta1().NuclioFunctions(function.Namespace).Update(function)
	return err
}

func (fo *functionOperator) getListWatcher(namespace string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return fo.controller.nuclioClientSet.NuclioV1beta1().NuclioFunctions(namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return fo.controller.nuclioClientSet.NuclioV1beta1().NuclioFunctions(namespace).Watch(options)
		},
	}
}

func (fo *functionOperator) getFunctionHTTPPort(functionResources functionres.Resources) (int, error) {
	var httpPort int

	service, err := functionResources.Service()
	if err != nil {
		return 0, errors.Wrap(err, "Failed to get function service")
	}

	if service != nil && len(service.Spec.Ports) != 0 {
		for _, port := range service.Spec.Ports {
			if port.Name == functionres.ContainerHTTPPortName {
				httpPort = int(port.NodePort)
				break
			}
		}
	}
	return httpPort, nil
}

func (fo *functionOperator) populateFunctionInvocationStatus(function *nuclioio.NuclioFunction,
	functionStatus *functionconfig.Status,
	functionResources functionres.Resources) error {

	// get function http port
	httpPort, err := fo.getFunctionHTTPPort(functionResources)
	if err != nil {
		return errors.Wrap(err, "Failed to get function http port")
	}

	service, err := functionResources.Service()
	if err != nil {
		return errors.Wrap(err, "Failed to get function service")
	}

	ingress, err := functionResources.Ingress()
	if err != nil {
		return errors.Wrap(err, "Failed to get function ingress")
	}

	functionStatus.HTTPPort = httpPort

	// add internal invocation urls
	functionStatus.InternalInvocationURLs = []string{}
	if service != nil {
		serviceHost, servicePort := client.GetDomainNameInvokeURL(service.GetName(), service.GetNamespace())
		functionStatus.InternalInvocationURLs = append(functionStatus.InternalInvocationURLs,
			fmt.Sprintf("%s:%d", serviceHost, servicePort))
	}

	// TODO: move the information on platformConfig and share with controller?
	// add external invocation url in form of "external-ip:nodeport"
	// first item is being filled by nuclio-dashboard to holds the information regarding the external ip address
	if len(function.Status.ExternalInvocationURLs) > 0 && service.Spec.Type == v1.ServiceTypeNodePort {
		hostPort := strings.Split(function.Status.ExternalInvocationURLs[0], ":")
		functionStatus.ExternalInvocationURLs = []string{fmt.Sprintf("%s:%d", hostPort[0], httpPort)}
	} else {
		functionStatus.ExternalInvocationURLs = []string{}
	}

	// add ingresses to external invocation urls
	if ingress != nil {
		for _, rule := range ingress.Spec.Rules {
			host := rule.Host
			path := "/"
			if rule.HTTP != nil {
				if len(rule.HTTP.Paths) > 0 {
					path = rule.HTTP.Paths[0].Path
				}
			}
			functionStatus.ExternalInvocationURLs = append(functionStatus.ExternalInvocationURLs,
				fmt.Sprintf("%s%s", host, path))
		}
	}
	return nil

}
