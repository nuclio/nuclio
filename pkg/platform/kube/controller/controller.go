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
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/functiondep"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/nuclio/logger"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Controller struct {
	logger            logger.Logger
	namespace         string
	restConfig        *rest.Config
	kubeClientSet         kubernetes.Interface
	nuclioClientSet   nuclioio_client.Interface
	functiondepClient functiondep.Client
	imagePullSecrets  string
}

func NewController(parentLogger logger.Logger,
	namespace string,
	imagePullSecrets string,
	kubeClientSet kubernetes.Interface,
	nuclioClientSet nuclioio_client.Interface,
	functiondepClient functiondep.Client) (*Controller, error) {

	// replace "*" with "", which is actually "all" in kube-speak
	if namespace == "*" {
		namespace = ""
	}

	newController := &Controller{
		logger:           parentLogger,
		namespace:        namespace,
		imagePullSecrets: imagePullSecrets,
		kubeClientSet: kubeClientSet,
		nuclioClientSet: nuclioClientSet,
		functiondepClient: functiondepClient,
	}

	// log version info
	version.Log(newController.logger)

	return newController, nil
}

func (c *Controller) Start() error {
	c.logger.InfoWith("Starting", "namespace", c.namespace)

	return nil
}

func (c *Controller) handleFunctionCRAdd(function *nuclioio.Function) error {
	var err error

	// try to add a function. if we're successful, we're done
	if err = c.addFunction(function); err == nil {
		return nil
	}

	// whatever the error, try to update the function CR
	c.logger.WarnWith("Failed to add function custom resource", "err", err)

	// indicate error state
	c.setFunctionState(function, functionconfig.FunctionStateError, errors.GetErrorStackString(err, 10))

	// try to update the function
	if updateFunctionErr := c.updateFunctioncr(function); updateFunctionErr != nil {
		c.logger.Warn("Failed to add function on validation failure")
	}

	return err
}

func (c *Controller) addFunction(function *nuclioio.Function) error {

	// if the function state is building, do nothing
	if function.Status.State == functionconfig.FunctionStateBuilding ||
		function.Status.State == functionconfig.FunctionStateError {
		c.logger.DebugWith("Function is building or in error, ignoring creation",
			"name", function.Name,
			"gen", function.ResourceVersion,
			"namespace", function.Namespace)

		return nil
	}

	c.logger.DebugWith("Adding function custom resource",
		"name", function.Name,
		"gen", function.ResourceVersion,
		"namespace", function.Namespace)

	// do some sanity
	if err := c.validateAddedFunctionCR(function); err != nil {
		return errors.Wrap(err, "Validation failed")
	}

	// update the deployment
	deployment, err := c.functiondepClient.CreateOrUpdate(function, c.imagePullSecrets)
	if err != nil {
		return errors.Wrap(err, "Failed to create deployment")
	}

	// wait for the deployment to become available
	if err = c.functiondepClient.WaitAvailable(deployment.Namespace, deployment.Name); err != nil {
		return errors.Wrap(err, "Failed to wait for deployment to be available")
	}

	// set the functioncr's state to be ready after the deployment becomes available
	c.setFunctionState(function, functionconfig.FunctionStateReady, "")

	// update the custom resource with all the labels and stuff
	if err = c.updateFunctioncr(function); err != nil {
		return errors.Wrap(err, "Failed to update function custom resource")
	}

	return nil
}

func (c *Controller) updateFunctioncr(function *nuclioio.Function) error {
	return nil
}

func (c *Controller) validateAddedFunctionCR(function *nuclioio.Function) error {
	if function.Spec.Runtime == "" {
		return errors.Errorf("Function must specify a runtime")
	}

	if function.Spec.Handler == "" {
		return errors.Errorf("Function must specify a handler")
	}

	return nil
}

func (c *Controller) handleFunctionCRUpdate(function *nuclioio.Function) error {
	var err error

	// try to update a function. if we're successful, we're done
	if err = c.updateFunction(function); err == nil {
		return nil
	}

	// whatever the error, try to update the function CR
	c.logger.WarnWith("Failed to update function custom resource", "err", err)

	c.setFunctionState(function, functionconfig.FunctionStateError, errors.GetErrorStackString(err, 10))

	// try to update the function
	if updateFunctionError := c.updateFunctioncr(function); updateFunctionError != nil {
		c.logger.Warn("Failed to add function on validation failure")
	}

	return err
}

func (c *Controller) updateFunction(function *nuclioio.Function) error {
	var err error

	// if the function state is building, do nothing
	if function.Status.State == functionconfig.FunctionStateBuilding ||
		function.Status.State == functionconfig.FunctionStateError {
		c.logger.DebugWith("Function is building or in error, ignoring update",
			"name", function.Name,
			"gen", function.ResourceVersion,
			"namespace", function.Namespace)

		return nil
	}

	c.logger.DebugWith("Updating function custom resource",
		"name", function.Name,
		"gen", function.ResourceVersion,
		"namespace", function.Namespace)

	// do some sanity
	if err = c.validateUpdatedFunctionCR(function); err != nil {
		return errors.Wrap(err, "Validation failed")
	}

	// indicate function is not yet ready
	c.setFunctionState(function, functionconfig.FunctionStateNotReady, "Updating resources")
	if c.updateFunctioncr(function) != nil {
		return errors.Wrap(err, "Failed to update function custom resource")
	}

	// update the deployment
	_, err = c.functiondepClient.CreateOrUpdate(function, "")
	if err != nil {
		return errors.Wrap(err, "Failed to create deployment")
	}

	// wait for the deployment to become available
	if err = c.functiondepClient.WaitAvailable(function.Namespace, function.Name); err != nil {
		return errors.Wrap(err, "Failed to wait for deployment to be available")
	}

	// indicate function is ready
	c.setFunctionState(function, functionconfig.FunctionStateReady, "")
	if err := c.updateFunctioncr(function); err != nil {
		return errors.Wrap(err, "Failed to update function custom resource")
	}

	return nil
}

func (c *Controller) validateUpdatedFunctionCR(function *nuclioio.Function) error {
	return nil
}

func (c *Controller) handleFunctionCRDelete(function *nuclioio.Function) error {
	c.logger.DebugWith("Function custom resource deleted",
		"name", function.Name,
		"gen", function.ResourceVersion,
		"namespace", function.Namespace)

	return c.functiondepClient.Delete(function.Namespace, function.Name)
}

func (c *Controller) setFunctionState(function *nuclioio.Function,
	state functionconfig.FunctionState,
	message string) {

	c.logger.DebugWith("Setting function state",
		"namespace", function.Namespace,
		"name", function.Name,
		"state", state,
		"message", message)

	function.Status.State = state
	function.Status.Message = message
}
