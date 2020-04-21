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
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"
	"github.com/nuclio/nuclio/pkg/platform/kube/operator"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/v3io/scaler-types"
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
		return fo.setFunctionError(nil, errors.New("Received unexpected object, expected function"))
	}

	// validate function name is according to k8s convention
	errorMessages := validation.IsQualifiedName(function.Name)
	if len(errorMessages) != 0 {
		joinedErrorMessage := strings.Join(errorMessages, ", ")
		return errors.New("Function name doesn't conform to k8s naming convention. Errors: " + joinedErrorMessage)
	}

	// only respond to functions which are either waiting for something or are in non-transitional state. We respond to
	// ready functions as part of controller resyncs, where we verify that a given function CRD has its resources
	// properly configured
	statesToRespond := []functionconfig.FunctionState{
		functionconfig.FunctionStateWaitingForResourceConfiguration,
		functionconfig.FunctionStateWaitingForScaleResourcesFromZero,
		functionconfig.FunctionStateWaitingForScaleResourcesToZero,
		functionconfig.FunctionStateReady,
		functionconfig.FunctionStateScaledToZero,
	}
	if !functionconfig.FunctionStateInSlice(function.Status.State, statesToRespond) {
		fo.logger.DebugWith("NuclioFunction is not waiting for resource creation or ready, skipping create/update",
			"name", function.Name,
			"state", function.Status.State,
			"namespace", function.Namespace)

		return nil
	}

	if functionconfig.ShouldSkipDeploy(function.Annotations) {
		fo.logger.InfoWith("Skipping function deploy",
			"name", function.Name,
			"state", function.Status.State,
			"namespace", function.Namespace)
		return fo.setFunctionStatus(function, &functionconfig.Status{
			State: functionconfig.FunctionStateImported,
		})
	}

	resources, err := fo.functionresClient.CreateOrUpdate(ctx, function, fo.imagePullSecrets)
	if err != nil {
		return fo.setFunctionError(function, errors.Wrap(err,
			"Failed to create/update function"))
	}

	// wait for up to 60 seconds or whatever was set in the spec
	readinessTimeout := function.Spec.ReadinessTimeoutSeconds
	if readinessTimeout == 0 {
		readinessTimeout = abstract.DefaultReadinessTimeoutSeconds
	}

	waitContext, cancel := context.WithDeadline(ctx, time.Now().Add(time.Duration(readinessTimeout)*time.Second))
	defer cancel()

	// wait until the function resources are ready
	if err = fo.functionresClient.WaitAvailable(waitContext, function.Namespace, function.Name); err != nil {
		return fo.setFunctionError(function, errors.Wrap(err,
			"Failed to wait for function resources to be available"))
	}

	var httpPort int

	service, err := resources.Service()
	if err != nil {
		return errors.Wrap(err, "Failed to get service")
	}

	if service != nil && len(service.Spec.Ports) != 0 {
		for _, port := range service.Spec.Ports {
			if port.Name == "http" {
				httpPort = int(port.NodePort)
				break
			}
		}
	}

	waitingStates := []functionconfig.FunctionState{
		functionconfig.FunctionStateWaitingForResourceConfiguration,
		functionconfig.FunctionStateWaitingForScaleResourcesFromZero,
		functionconfig.FunctionStateWaitingForScaleResourcesToZero,
	}

	if functionconfig.FunctionStateInSlice(function.Status.State, waitingStates) {

		var scaleEvent scaler_types.ScaleEvent
		var finalState functionconfig.FunctionState
		switch function.Status.State {
		case functionconfig.FunctionStateWaitingForScaleResourcesToZero:
			scaleEvent = scaler_types.ScaleToZeroCompletedScaleEvent
			finalState = functionconfig.FunctionStateScaledToZero
		case functionconfig.FunctionStateWaitingForScaleResourcesFromZero:
			scaleEvent = scaler_types.ScaleFromZeroCompletedScaleEvent
			finalState = functionconfig.FunctionStateReady
		case functionconfig.FunctionStateWaitingForResourceConfiguration:
			scaleEvent = scaler_types.ResourceUpdatedScaleEvent
			finalState = functionconfig.FunctionStateReady
		}

		functionStatus := &functionconfig.Status{
			State:    finalState,
			HTTPPort: httpPort,
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
	scaleToZeroEvent scaler_types.ScaleEvent) error {

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

func (fo *functionOperator) setFunctionError(function *nuclioio.NuclioFunction, err error) error {

	// whatever the error, try to update the function CR
	fo.logger.WarnWith("Setting function error", "name", function.Name, "err", err)

	if fo.setFunctionStatus(function, &functionconfig.Status{
		State:   functionconfig.FunctionStateError,
		Message: errors.GetErrorStackString(err, 10),
	}) != nil {
		fo.logger.Warn("Failed to update function on error")
	}

	return err
}

func (fo *functionOperator) setFunctionStatus(function *nuclioio.NuclioFunction, status *functionconfig.Status) error {

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
