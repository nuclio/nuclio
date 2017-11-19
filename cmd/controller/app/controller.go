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

package app

import (
	"fmt"
	"strconv"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform/kube/controller"
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"
	"github.com/nuclio/nuclio/pkg/platform/kube/functiondep"
	"github.com/nuclio/nuclio/pkg/version"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio-sdk"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	latestTag = "latest"
)

type Controller struct {
	logger                   nuclio.Logger
	namespace                string
	restConfig               *rest.Config
	clientSet                *kubernetes.Clientset
	functioncrClient         functioncrClient
	functioncrChangesChan    chan functioncr.Change
	functiondepClient        functiondepClient
	ignoredFunctionCRChanges changeIgnorer
}

func NewController(namespace string, configurationPath string) (*Controller, error) {
	var err error

	newController := &Controller{
		namespace:             namespace,
		functioncrChangesChan: make(chan functioncr.Change),
	}

	newController.logger, err = newController.createLogger()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create logger")
	}

	newController.logger.InfoWith("Starting", "namespace", namespace)

	// log version info
	version.Log(newController.logger)

	// holds changes that the controller itself triggered and needs to ignore
	newController.ignoredFunctionCRChanges = controller.NewIgnoredChanges(newController.logger)

	newController.restConfig, err = newController.getClientConfig(configurationPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get client configuration")
	}

	newController.clientSet, err = kubernetes.NewForConfig(newController.restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create client set")
	}

	// create a client for function custom resources
	newController.functioncrClient, err = functioncr.NewClient(newController.logger,
		newController.restConfig,
		newController.clientSet)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function custom resource client")
	}

	// create a client for function deployments
	newController.functiondepClient, err = functiondep.NewClient(newController.logger,
		newController.clientSet)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function deployment client")
	}

	return newController, nil
}

func (c *Controller) Start() error {

	// ensure that the "functions" third party resource exists in kubernetes
	err := c.functioncrClient.CreateResource()
	if err != nil {
		return errors.Wrap(err, "Failed to create custom resource object")
	}

	// list all existing function custom resources and add their versions to the list
	// of ignored versions. this is because the watcher will trigger them as if they
	// were updated
	if err = c.populateInitialFunctionCRIgnoredChanges(); err != nil {
		return errors.Wrap(err, "Failed to populate initial ignored function cr changes")
	}

	// wait for changes on the function custom resource
	c.functioncrClient.WatchForChanges(c.namespace, c.functioncrChangesChan)

	for {
		functionChange := <-c.functioncrChangesChan

		// check if this change should be ignored
		if c.ignoredFunctionCRChanges.Pop(functionChange.Function.GetNamespacedName(),
			functionChange.Function.ResourceVersion) {

			c.logger.DebugWith("Ignoring change")

			continue
		}

		switch functionChange.Kind {
		case functioncr.ChangeKindAdded:
			err = c.handleFunctionCRAdd(functionChange.Function)
		case functioncr.ChangeKindUpdated:
			err = c.handleFunctionCRUpdate(functionChange.Function)
		case functioncr.ChangeKindDeleted:
			err = c.handleFunctionCRDelete(functionChange.Function)
		default:
			err = fmt.Errorf("Unknown change kind: %d", functionChange.Kind)
		}

		if err != nil {
			c.logger.ErrorWith("Failed to handle function change",
				"kind", functionChange.Kind,
				"err", err)
		}
	}
}

func (c *Controller) getClientConfig(configurationPath string) (*rest.Config, error) {
	if configurationPath != "" {
		return clientcmd.BuildConfigFromFlags("", configurationPath)
	}

	return rest.InClusterConfig()
}

func (c *Controller) createLogger() (nuclio.Logger, error) {

	// TODO: configuration stuff
	return nucliozap.NewNuclioZapCmd("controller", nucliozap.DebugLevel)
}

func (c *Controller) handleFunctionCRAdd(function *functioncr.Function) error {
	var err error

	// try to add a function. if we're successful, we're done
	if err = c.addFunction(function); err == nil {
		return nil
	}

	// whatever the error, try to update the function CR
	c.logger.WarnWith("Failed to add function custom resource", "err", err)

	function.SetStatus(functioncr.FunctionStateError, err.Error())

	// try to update the function
	if updateFunctionErr := c.updateFunctioncr(function); updateFunctionErr != nil {
		c.logger.Warn("Failed to add function on validation failure")
	}

	return err
}

func (c *Controller) addFunction(function *functioncr.Function) error {
	c.logger.DebugWith("Adding function custom resource",
		"name", function.Name,
		"gen", function.ResourceVersion,
		"namespace", function.Namespace)

	// do some sanity
	if err := c.validateAddedFunctionCR(function); err != nil {
		return errors.Wrap(err, "Validation failed")
	}

	// get the function name and version
	functionName, _, err := function.GetNameAndVersion()
	if err != nil {

		// should never happen since this is validated in validateAddedFunctionCR, but check anyway
		return errors.Wrap(err, "Failed to get function name an version")
	}

	// add labels
	functionLabels := function.GetLabels()
	functionLabels["name"] = functionName
	functionLabels["version"] = latestTag

	// set version and alias
	function.Spec.Version = -1
	function.Spec.Alias = latestTag

	// if we need to publish the function, do that
	if function.Spec.Publish {
		function.Spec.Publish = false
		function.Spec.Version++

		err = c.publishFunction(function)
		if err != nil {
			return errors.Wrap(err, "Failed to publish function")
		}
	}

	// update the custom resource with all the labels and stuff
	function.SetStatus(functioncr.FunctionStateProcessed, "")
	if err = c.updateFunctioncr(function); err != nil {
		return errors.Wrap(err, "Failed to update function custom resource")
	}

	// update the deployment
	_, err = c.functiondepClient.CreateOrUpdate(function)
	if err != nil {
		return errors.Wrap(err, "Failed to create deployment")
	}

	return nil
}

func (c *Controller) updateFunctioncr(function *functioncr.Function) error {
	updatedFunction, err := c.functioncrClient.Update(function)
	if err != nil {
		return errors.Wrap(err, "Failed to update function custom resource")
	}

	// we'll be getting a notification about the update we just did - ignore it
	c.ignoredFunctionCRChanges.Push(updatedFunction.GetNamespacedName(), updatedFunction.ResourceVersion)

	return nil
}

func (c *Controller) publishFunction(function *functioncr.Function) error {
	publishedFunction := *function
	publishedFunction.Labels = nil

	c.logger.InfoWith("Publishing function", "function", function)

	// update the function name
	publishedFunction.Name = fmt.Sprintf("%s%s%d",
		publishedFunction.Name, functioncr.GetVersionSeparator(), publishedFunction.Spec.Version)

	// clear version and alias
	publishedFunction.ResourceVersion = ""
	publishedFunction.Spec.Alias = ""
	publishedFunction.Status.State = functioncr.FunctionStateProcessed

	// update version to that of the spec (it's not latest anymore)
	publishedFunction.GetLabels()["name"] = publishedFunction.Name
	publishedFunction.GetLabels()["version"] = strconv.Itoa(publishedFunction.Spec.Version)

	// create the function
	createdPublishedFunction, err := c.functioncrClient.Create(&publishedFunction)
	if err != nil {
		return errors.Wrap(err, "Failed to create function CR")
	}

	// ignore the trigger since we don't want to apply the same validation we do to user functions to stuff we create
	c.ignoredFunctionCRChanges.Push(createdPublishedFunction.GetNamespacedName(),
		createdPublishedFunction.ResourceVersion)

	// create the deployment
	_, err = c.functiondepClient.CreateOrUpdate(&publishedFunction)
	if err != nil {
		return errors.Wrap(err, "Failed to create deployment for published function")
	}

	return err
}

func (c *Controller) validateAddedFunctionCR(function *functioncr.Function) error {
	_, functionVersion, err := function.GetNameAndVersion()
	if err != nil {
		return errors.Wrap(err, "Failed to get name and version from function name")
	}

	if functionVersion != nil {
		return errors.Errorf("Cannot specify function version in name on a created function (%d)", functionVersion)
	}

	if function.Spec.Version != 0 {
		return errors.Errorf("Cannot specify function version in spec on a created function (%d)", function.Spec.Version)
	}

	if function.Spec.Alias != "" {
		return errors.Errorf("Cannot specify alias on a created function (%s)", function.Spec.Alias)
	}

	if function.Spec.Runtime == "" {
		return errors.Errorf("Function must specify a runtime")
	}

	if function.Spec.Handler == "" {
		return errors.Errorf("Function must specify a handler")
	}

	return nil
}

func (c *Controller) handleFunctionCRUpdate(function *functioncr.Function) error {
	var err error

	// try to update a function. if we're successful, we're done
	if err = c.updateFunction(function); err == nil {
		return nil
	}

	// whatever the error, try to update the function CR
	c.logger.WarnWith("Failed to update function custom resource", "err", err)

	function.SetStatus(functioncr.FunctionStateError, err.Error())

	// try to update the function
	if updateFunctionError := c.updateFunctioncr(function); updateFunctionError != nil {
		c.logger.Warn("Failed to add function on validation failure")
	}

	return err
}

func (c *Controller) updateFunction(function *functioncr.Function) error {
	var err error

	c.logger.DebugWith("Updating function custom resource",
		"name", function.Name,
		"gen", function.ResourceVersion,
		"namespace", function.Namespace)

	// do some sanity
	if err = c.validateUpdatedFunctionCR(function); err != nil {
		return errors.Wrap(err, "Validation failed")
	}

	// if we need to publish the function, do that
	if function.Spec.Publish {
		function.Spec.Publish = false
		function.Spec.Version++

		err = c.publishFunction(function)
		if err != nil {
			return errors.Wrap(err, "Failed to publish function")
		}
	}

	// update the custom resource with all the labels and stuff
	function.SetStatus(functioncr.FunctionStateProcessed, "")
	if c.updateFunctioncr(function) != nil {
		return errors.Wrap(err, "Failed to update function custom resource")
	}

	// update the deployment
	_, err = c.functiondepClient.CreateOrUpdate(function)
	if err != nil {
		return errors.Wrap(err, "Failed to create deployment")
	}

	return nil
}

func (c *Controller) validateUpdatedFunctionCR(function *functioncr.Function) error {
	_, functionVersion, err := function.GetNameAndVersion()
	if err != nil {
		return errors.Wrap(err, "Failed to get name and version from function name")
	}

	if function.Spec.Alias != latestTag && functionVersion == nil {
		return errors.Errorf("Cannot update alias on non-published version")
	}

	if function.Spec.Publish && functionVersion != nil {
		return errors.Errorf("Cannot publish an already published version")
	}

	return nil
}

func (c *Controller) handleFunctionCRDelete(function *functioncr.Function) error {
	c.logger.DebugWith("Function custom resource deleted",
		"name", function.Name,
		"gen", function.ResourceVersion,
		"namespace", function.Namespace)

	return c.functiondepClient.Delete(function.Namespace, function.Name)
}

func (c *Controller) populateInitialFunctionCRIgnoredChanges() error {
	functionCRs, err := c.functioncrClient.List(c.namespace, &meta_v1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to list function custom resources")
	}

	// iterate over function CRs
	for _, functionCR := range functionCRs.Items {
		c.ignoredFunctionCRChanges.Push(functionCR.GetNamespacedName(), functionCR.ResourceVersion)
	}

	return nil
}
