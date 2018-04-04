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
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"
	"github.com/nuclio/nuclio/pkg/platform/kube/operator"

	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	functionresClient functionres.Client) (*functionOperator, error) {
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
		4,
		newFunctionOperator.getListWatcher(controller.namespace),
		&nuclioio.Function{},
		resyncInterval,
		newFunctionOperator)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function operator")
	}

	return newFunctionOperator, nil
}

// CreateOrUpdate handles creation/update of an object
func (fo *functionOperator) CreateOrUpdate(object runtime.Object) error {
	function, objectIsFunction := object.(*nuclioio.Function)
	if !objectIsFunction {
		return fo.setFunctionError(nil, errors.New("Received unexpected object, expected function"))
	}

	// only respond to functions which are either waiting for resource configuration or are ready. We respond to
	// ready functions as part of controller resyncs, where we verify that a given function CRD has its resources
	// properly configured
	if function.Status.State != functionconfig.FunctionStateWaitingForResourceConfiguration &&
		function.Status.State != functionconfig.FunctionStateReady {
		fo.logger.DebugWith("Function is not waiting for resource creation or ready, skipping create/update",
			"name", function.Name,
			"state", function.Status.State,
			"namespace", function.Namespace)

		return nil
	}

	resources, err := fo.functionresClient.CreateOrUpdate(function, fo.imagePullSecrets)
	if err != nil {
		return fo.setFunctionError(function, errors.Wrap(err,
			"Failed to create/update function"))
	}

	// wait until the function resources are ready
	if err = fo.functionresClient.WaitAvailable(function.Namespace, function.Name); err != nil {
		return fo.setFunctionError(function, errors.Wrap(err,
			"Failed to wait for function resources to be available"))
	}

	var httpPort int

	service, err := resources.Service()
	if err != nil {
		return errors.Wrap(err, "Failed to get service")
	}

	if service != nil && len(service.Spec.Ports) != 0 {
		httpPort = int(service.Spec.Ports[0].NodePort)
	}

	// if the function state was ready, don't re-write the function state
	if function.Status.State != functionconfig.FunctionStateReady {
		return fo.setFunctionStatus(function, &functionconfig.Status{
			State:    functionconfig.FunctionStateReady,
			HTTPPort: httpPort,
		})
	}

	return nil
}

// Delete handles delete of an object
func (fo *functionOperator) Delete(namespace string, name string) error {
	fo.logger.DebugWith("Deleting function",
		"name", name,
		"namespace", namespace)

	return fo.functionresClient.Delete(namespace, name)
}

func (fo *functionOperator) start() error {
	go fo.operator.Start() // nolint: errcheck

	return nil
}

func (fo *functionOperator) setFunctionError(function *nuclioio.Function, err error) error {

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

func (fo *functionOperator) setFunctionStatus(function *nuclioio.Function, status *functionconfig.Status) error {

	fo.logger.DebugWith("Setting function state", "name", function.Name, "status", status)

	// indicate error state
	function.Status = *status

	// try to update the function
	_, err := fo.controller.nuclioClientSet.NuclioV1beta1().Functions(function.Namespace).Update(function)
	return err
}

func (fo *functionOperator) getListWatcher(namespace string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return fo.controller.nuclioClientSet.NuclioV1beta1().Functions(namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return fo.controller.nuclioClientSet.NuclioV1beta1().Functions(namespace).Watch(options)
		},
	}
}
