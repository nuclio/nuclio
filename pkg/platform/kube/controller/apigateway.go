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

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"github.com/nuclio/nuclio/pkg/platform/kube/operator"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type apiGatewayOperator struct {
	logger     logger.Logger
	controller *Controller
	operator   operator.Operator
}

func newAPIGatewayOperator(ctx context.Context,
	parentLogger logger.Logger,
	controller *Controller,
	resyncInterval *time.Duration,
	numWorkers int) (*apiGatewayOperator, error) {
	var err error

	loggerInstance := parentLogger.GetChild("apigateway")

	newAPIGatewayOperator := &apiGatewayOperator{
		logger:     loggerInstance,
		controller: controller,
	}

	// create an api gateway operator
	newAPIGatewayOperator.operator, err = operator.NewMultiWorker(ctx,
		loggerInstance,
		numWorkers,
		newAPIGatewayOperator.getListWatcher(ctx, controller.namespace),
		&nuclioio.NuclioAPIGateway{},
		resyncInterval,
		newAPIGatewayOperator)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create api gateway operator")
	}

	parentLogger.DebugWithCtx(ctx, "Created api gateway operator",
		"numWorkers", numWorkers,
		"resyncInterval", resyncInterval)

	return newAPIGatewayOperator, nil
}

// CreateOrUpdate handles creation/update of an object
func (ago *apiGatewayOperator) CreateOrUpdate(ctx context.Context, object runtime.Object) error {
	var err error

	apiGateway, objectIsAPIGateway := object.(*nuclioio.NuclioAPIGateway)
	if !objectIsAPIGateway {
		return errors.New("Received unexpected object, expected api gateway")
	}

	// validate the state is inside states to respond to
	if !ago.shouldRespondToState(apiGateway.Status.State) {
		ago.logger.DebugWithCtx(ctx, "Api gateway state is not waiting for creation/update, skipping create/update",
			"name", apiGateway.Spec.Name,
			"state", apiGateway.Status.State)
		return nil
	}

	apiGateway.Status.Description = apiGateway.Spec.Description

	if apiGateway.Labels == nil {
		apiGateway.Labels = map[string]string{}
	}

	// set default project-name if none given
	if apiGateway.Labels[common.NuclioResourceLabelKeyProjectName] == "" {
		apiGateway.Labels[common.NuclioResourceLabelKeyProjectName] = platform.DefaultProjectName
	}

	// validate api gateway name is according to k8s convention
	errorMessages := validation.IsQualifiedName(apiGateway.Name)
	if len(errorMessages) != 0 {
		joinedErrorMessage := strings.Join(errorMessages, ", ")
		return errors.Errorf("Api gateway name doesn't conform to k8s naming convention. Errors: %s", joinedErrorMessage)
	}

	// create/update the api gateway
	if _, err = ago.controller.apigatewayresClient.CreateOrUpdate(ctx, apiGateway); err != nil {
		ago.logger.WarnWithCtx(ctx, "Failed to create/update api gateway. Updating state accordingly")
		if err := ago.setAPIGatewayState(ctx, apiGateway, platform.APIGatewayStateError, err); err != nil {
			ago.logger.WarnWithCtx(ctx, "Failed to set api gateway state as error", "err", err)
		}

		return errors.Wrap(err, "Failed to create/update api gateway")
	}

	// wait for api gateway to become available
	ago.controller.apigatewayresClient.WaitAvailable(ctx, apiGateway.Namespace, apiGateway.Name)

	// set state to ready
	if err := ago.setAPIGatewayState(ctx, apiGateway, platform.APIGatewayStateReady, nil); err != nil {
		return errors.Wrap(err, "Failed to set api gateway state after it was successfully created")
	}

	ago.logger.DebugWithCtx(ctx, "Successfully created/updated api gateway", "apiGateway", apiGateway)

	return nil
}

// Delete handles delete of an object
func (ago *apiGatewayOperator) Delete(ctx context.Context, namespace string, name string) error {
	ago.controller.apigatewayresClient.Delete(ctx, namespace, name)
	return nil
}

func (ago *apiGatewayOperator) getListWatcher(ctx context.Context, namespace string) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return ago.controller.nuclioClientSet.NuclioV1beta1().NuclioAPIGateways(namespace).List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return ago.controller.nuclioClientSet.NuclioV1beta1().NuclioAPIGateways(namespace).Watch(ctx, options)
		},
	}
}

func (ago *apiGatewayOperator) shouldRespondToState(state platform.APIGatewayState) bool {
	statesToRespond := []string{
		string(platform.APIGatewayStateWaitingForProvisioning),
		string(platform.APIGatewayStateNone),
	}

	return common.StringSliceContainsString(statesToRespond, string(state))
}

func (ago *apiGatewayOperator) setAPIGatewayState(ctx context.Context,
	apiGateway *nuclioio.NuclioAPIGateway,
	state platform.APIGatewayState,
	lastError error) error {
	ago.logger.DebugWithCtx(ctx, "Setting api gateway state", "name", apiGateway.Name, "state", state)

	apiGateway.Status.State = state

	// if a last error was passed, set it
	if lastError != nil {
		apiGateway.Status.LastError = lastError.Error()
	}

	// try to update the api gateway with the new state
	_, err := ago.controller.nuclioClientSet.NuclioV1beta1().NuclioAPIGateways(apiGateway.Namespace).Update(ctx,
		apiGateway,
		metav1.UpdateOptions{})
	return err
}

func (ago *apiGatewayOperator) start(ctx context.Context) error {
	go ago.operator.Start(ctx) // nolint: errcheck

	return nil
}
