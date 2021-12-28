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
	"time"

	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/operator"

	"github.com/nuclio/errors"
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

func newFunctionEventOperator(ctx context.Context,
	parentLogger logger.Logger,
	controller *Controller,
	resyncInterval *time.Duration,
	numWorkers int) (*functionEventOperator, error) {
	var err error

	loggerInstance := parentLogger.GetChild("function_event")

	newFunctionEventOperator := &functionEventOperator{
		logger:     loggerInstance,
		controller: controller,
	}

	// create a function event operator
	newFunctionEventOperator.operator, err = operator.NewMultiWorker(ctx,
		loggerInstance,
		numWorkers,
		newFunctionEventOperator.getListWatcher(ctx, controller.namespace),
		&nuclioio.NuclioFunctionEvent{},
		resyncInterval,
		newFunctionEventOperator)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function event operator")
	}

	parentLogger.DebugWithCtx(ctx, "Created function event operator",
		"numWorkers", numWorkers,
		"resyncInterval", resyncInterval)

	return newFunctionEventOperator, nil
}

// CreateOrUpdate handles creation/update of an object
func (feo *functionEventOperator) CreateOrUpdate(ctx context.Context, object runtime.Object) error {
	functionEvent, objectIsFunctionEvent := object.(*nuclioio.NuclioFunctionEvent)
	if !objectIsFunctionEvent {
		return errors.New("Received unexpected object, expected function event")
	}

	feo.logger.DebugWithCtx(ctx, "Created/updated", "functionEventName", functionEvent.Name)
	return nil
}

// Delete handles delete of an object
func (feo *functionEventOperator) Delete(ctx context.Context, namespace string, name string) error {
	feo.logger.DebugWithCtx(ctx, "Deleted", "namespace", namespace, "name", name)

	return nil
}

func (feo *functionEventOperator) getListWatcher(ctx context.Context, namespace string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return feo.controller.nuclioClientSet.NuclioV1beta1().NuclioFunctionEvents(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return feo.controller.nuclioClientSet.NuclioV1beta1().NuclioFunctionEvents(namespace).Watch(ctx, options)
		},
	}
}

func (feo *functionEventOperator) start(ctx context.Context) error {
	go feo.operator.Start(ctx) // nolint: errcheck

	return nil
}
