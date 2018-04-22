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
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/operator"

	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type functionEventOperator struct {
	logger     logger.Logger
	controller *Controller
	operator   operator.Operator
}

func newFunctionEventOperator(parentLogger logger.Logger,
	controller *Controller,
	resyncInterval *time.Duration) (*functionEventOperator, error) {
	var err error

	loggerInstance := parentLogger.GetChild("function_event")

	newFunctionEventOperator := &functionEventOperator{
		logger:     loggerInstance,
		controller: controller,
	}

	// create a function event operator
	newFunctionEventOperator.operator, err = operator.NewMultiWorker(loggerInstance,
		2,
		newFunctionEventOperator.getListWatcher(controller.namespace),
		&nuclioio.FunctionEvent{},
		resyncInterval,
		newFunctionEventOperator)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function event operator")
	}

	return newFunctionEventOperator, nil
}

func (feo *functionEventOperator) getListWatcher(namespace string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return feo.controller.nuclioClientSet.NuclioV1beta1().FunctionEvents(namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return feo.controller.nuclioClientSet.NuclioV1beta1().FunctionEvents(namespace).Watch(options)
		},
	}
}

// CreateOrUpdate handles creation/update of an object
func (feo *functionEventOperator) CreateOrUpdate(object runtime.Object) error {
	feo.logger.DebugWith("Created/updated", "object", object)

	return nil
}

// Delete handles delete of an object
func (feo *functionEventOperator) Delete(namespace string, name string) error {
	feo.logger.DebugWith("Deleted", "namespace", namespace, "name", name)

	return nil
}

func (feo *functionEventOperator) start() error {
	go feo.operator.Start() // nolint: errcheck

	return nil
}
