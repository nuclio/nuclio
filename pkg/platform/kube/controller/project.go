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
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/operator"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type projectOperator struct {
	logger     logger.Logger
	controller *Controller
	operator   operator.Operator
}

func newProjectOperator(ctx context.Context,
	parentLogger logger.Logger,
	controller *Controller,
	resyncInterval *time.Duration,
	numWorkers int) (*projectOperator, error) {
	var err error

	loggerInstance := parentLogger.GetChild("project")

	newProjectOperator := &projectOperator{
		logger:     loggerInstance,
		controller: controller,
	}

	// create a project operator
	newProjectOperator.operator, err = operator.NewMultiWorker(ctx,
		loggerInstance,
		numWorkers,
		newProjectOperator.getListWatcher(ctx, controller.namespace),
		&nuclioio.NuclioProject{},
		resyncInterval,
		newProjectOperator)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create project operator")
	}

	parentLogger.DebugWithCtx(ctx, "Created project operator",
		"numWorkers", numWorkers,
		"resyncInterval", resyncInterval)

	return newProjectOperator, nil
}

// CreateOrUpdate handles creation/update of an object
func (po *projectOperator) CreateOrUpdate(ctx context.Context, object runtime.Object) error {
	project, objectIsProject := object.(*nuclioio.NuclioProject)
	if !objectIsProject {
		return errors.New("Received unexpected object, expected project")
	}

	po.logger.DebugWithCtx(ctx, "Created/updated", "projectName", project.Name)
	return nil
}

// Delete handles delete of an object
func (po *projectOperator) Delete(ctx context.Context, namespace string, name string) error {
	po.logger.InfoWithCtx(ctx, "Deleting project resources", "namespace", namespace, "projectName", name)

	projectNameLabelSelector := fmt.Sprintf("%s=%s", common.NuclioResourceLabelKeyProjectName, name)

	// delete api gateways
	if err := po.controller.nuclioClientSet.
		NuclioV1beta1().
		NuclioAPIGateways(namespace).
		DeleteCollection(ctx, metav1.DeleteOptions{},
			metav1.ListOptions{
				LabelSelector: projectNameLabelSelector,
			}); err != nil {
		po.logger.WarnWithCtx(ctx, "Failed to delete project api gateway",
			"namespace", namespace,
			"projectName", name,
			"err", err)
		return errors.Wrap(err, "Failed to delete project api gateways")
	}

	// NOTE: functions delete their related function events
	// delete nuclio functions
	if err := po.controller.nuclioClientSet.
		NuclioV1beta1().
		NuclioFunctions(namespace).
		DeleteCollection(ctx, metav1.DeleteOptions{},
			metav1.ListOptions{
				LabelSelector: projectNameLabelSelector,
			}); err != nil {

		po.logger.WarnWithCtx(ctx, "Failed to delete project functions",
			"namespace", namespace,
			"projectName", name,
			"err", err)
		return errors.Wrap(err, "Failed to delete project functions")
	}

	// done
	po.logger.DebugWithCtx(ctx, "Successfully deleted project resources",
		"namespace", namespace,
		"projectName", name)
	return nil
}

func (po *projectOperator) getListWatcher(ctx context.Context, namespace string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return po.controller.nuclioClientSet.NuclioV1beta1().NuclioProjects(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return po.controller.nuclioClientSet.NuclioV1beta1().NuclioProjects(namespace).Watch(ctx, options)
		},
	}
}

func (po *projectOperator) start(ctx context.Context) error {
	go po.operator.Start(ctx) // nolint: errcheck

	return nil
}
