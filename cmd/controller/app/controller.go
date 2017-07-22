package app

import (
	"fmt"
	"strconv"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/functiondep"
	"github.com/nuclio/nuclio/pkg/zap"

	"github.com/nuclio/nuclio/pkg/controller"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Controller struct {
	logger                   nuclio.Logger
	restConfig               *rest.Config
	clientSet                *kubernetes.Clientset
	functioncrClient         *functioncr.Client
	functioncrChangesChan    chan functioncr.Change
	functiondepClient        *functiondep.Client
	ignoredFunctionCRChanges *controller.IgnoredChanges
}

func NewController(configurationPath string) (*Controller, error) {
	var err error

	newController := &Controller{
		functioncrChangesChan: make(chan functioncr.Change),
	}

	newController.logger, err = newController.createLogger()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create logger")
	}

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
	var err error

	// ensure that the "functions" third party resource exists in kubernetes
	err = c.functioncrClient.CreateResource()
	if err != nil {
		return errors.Wrap(err, "Failed to create custom resource object")
	}

	// list all existing function custom resources and add their versions to the list
	// of ignored versions. this is because the watcher will trigger them as if they
	// were udpated
	if err := c.populateInitialFunctionCRIgnoredChanges(); err != nil {
		return errors.Wrap(err, "Failed to populate initial ignored function cr changes")
	}

	// wait for changes on the function custom resource
	c.functioncrClient.WatchForChanges(c.functioncrChangesChan)

	for {
		functionChange := <-c.functioncrChangesChan

		// check if this change should be ignored
		if c.ignoredFunctionCRChanges.Pop(functionChange.Function.GetNamespacedName(),
			functionChange.Function.ResourceVersion) {

			c.logger.DebugWith("Ignoring change")

			continue
		}

		switch functionChange.Kind {
		case functioncr.ChangeKindUpdated, functioncr.ChangeKindAdded:
			err = c.handleFunctionCRAddOrUpdate(functionChange.Function)
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
	return nucliozap.NewNuclioZap("controller", nucliozap.DebugLevel)
}

func (c *Controller) handleFunctionCRAddOrUpdate(function *functioncr.Function) error {
	var err error

	c.logger.DebugWith("Function custom resource added/updated",
		"name", function.Name,
		"gen", function.ResourceVersion,
		"namespace", function.Namespace)

	// do some sanity
	if err := c.validateCreatedUpdatedFunctionCR(function); err != nil {
		return errors.Wrap(err, "Can't create/update function - validation failed")
	}

	// get the function name and version
	functionName, _ := function.GetNameAndVersion()
	functionLabels := function.GetLabels()

	// update function name
	functionLabels["function"] = functionName

	// set version
	if function.Spec.Version == 0 {
		function.Spec.Version = 1
		function.Spec.Alias = "latest"
	}

	if function.Spec.Alias == "latest" {
		functionLabels["version"] = "latest"
	} else {
		functionLabels["version"] = strconv.Itoa(function.Spec.Version)
	}

	// update the custom resource with all the labels and stuff
	function.SetStatus(functioncr.FunctionStateProcessed, "")
	if c.updateFunctionCR(function) != nil {
		return errors.Wrap(err, "Failed to update function custom resource")
	}

	// create or update the deployment
	_, err = c.functiondepClient.CreateOrUpdate(function)
	if err != nil {
		return errors.Wrap(err, "Failed to create deployment")
	}

	return nil
}

func (c *Controller) updateFunctionCR(function *functioncr.Function) error {
	updatedFunction, err := c.functioncrClient.Update(function)
	if err != nil {
		return errors.Wrap(err, "Failed to update function custom resource")
	}

	// we'll be getting a notification about the update we just did - ignore it
	c.ignoredFunctionCRChanges.Push(updatedFunction.GetNamespacedName(), updatedFunction.ResourceVersion)

	return nil
}

func (c *Controller) validateCreatedUpdatedFunctionCR(function *functioncr.Function) error {
	functionName, functionVersion := function.GetNameAndVersion()

	setFunctionError := func(message string) error {
		function.SetStatus(functioncr.FunctionStateError, message)

		// try to update the function
		if err := c.updateFunctionCR(function); err != nil {
			c.logger.Warn("Failed to update function on validation failure")
		}

		return fmt.Errorf("Validation failure: %s", message)
	}

	if function.Labels["function"] != "" && functionName != function.Labels["function"] {
		return setFunctionError("Name and function label must be the same")
	}

	if functionVersion > 0 && function.Spec.Version != functionVersion {
		return setFunctionError("Version number cannot be modified on published versions")
	}

	if functionVersion > 0 && function.Spec.Alias == "latest" {
		return setFunctionError(`Older versions cannot be tagged as "latest"`)
	}

	if functionVersion > 0 && function.Spec.Alias != "latest" && !function.Spec.Publish {
		return setFunctionError(`Head version must be tagged as 'latest' or use Publish flag`)
	}

	if (function.Spec.Image == "" || function.Spec.Disabled) && function.Spec.Publish {
		return setFunctionError("Can't Publish on build or disabled function")
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
	functionCRs, err := c.functioncrClient.List("")
	if err != nil {
		return errors.Wrap(err, "Failed to list function custom resources")
	}

	// iterate over function CRs
	for _, functionCR := range functionCRs.Items {
		c.ignoredFunctionCRChanges.Push(functionCR.GetNamespacedName(), functionCR.ResourceVersion)
	}

	return nil
}
