package app

import (
	"fmt"

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
	functioncrClient         functioncrClient
	functioncrChangesChan    chan functioncr.Change
	functiondepClient        functiondepClient
	ignoredFunctionCRChanges changeIgnorer
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
	return nucliozap.NewNuclioZap("controller", nucliozap.DebugLevel)
}

func (c *Controller) handleFunctionCRAdd(function *functioncr.Function) error {

	// do the add and catch errors
	err := c.addFunctioncr(function)

	// whatever the error, try to update the function CR
	if err != nil {
		function.SetStatus(functioncr.FunctionStateError, err.Error())

		// try to update the function
		if err := c.updateFunctionCR(function); err != nil {
			c.logger.Warn("Failed to update function on validation failure")
		}
	}

	return nil
}

func (c *Controller) addFunctioncr(function *functioncr.Function) error {
	var err error

	c.logger.DebugWith("Function custom resource added",
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

	// save whether to publish and make sure publish is set to false
	publish := function.Spec.Publish
	function.Spec.Publish = false

	// add labels
	functionLabels := function.GetLabels()
	functionLabels["name"] = functionName
	functionLabels["version"] = "latest"

	// set version and alias
	function.Spec.Version = 0
	function.Spec.Alias = "latest"

	// update the custom resource with all the labels and stuff
	function.SetStatus(functioncr.FunctionStateProcessed, "")
	if c.updateFunctionCR(function) != nil {
		return errors.Wrap(err, "Failed to update function custom resource")
	}

	// create the deployment
	_, err = c.functiondepClient.CreateOrUpdate(function)
	if err != nil {
		return errors.Wrap(err, "Failed to create deployment")
	}

	// if we need to publish the function, do that
	if publish {
		err = c.publishFunction(function)
		if err != nil {
			return errors.Wrap(err, "Failed to publish function")
		}
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

func (c *Controller) publishFunction(function *functioncr.Function) error {
	publishedFunction := *function

	// update the function name
	publishedFunction.Name = fmt.Sprintf("%s-%d",
		publishedFunction.Name, publishedFunction.Spec.Version)


	// create the function
	_, err := c.functioncrClient.Create(&publishedFunction)

	return err
}

func (c *Controller) validateAddedFunctionCR(function *functioncr.Function) error {
	_, functionVersion, err := function.GetNameAndVersion()
	if err != nil {
		return errors.Wrap(err, "Failed to get name and version from function name")
	}

	if functionVersion != 0 {
		return errors.Errorf("Cannot specify function version in name on a created function (%d)", functionVersion)
	}

	if function.Spec.Version != 0 {
		return errors.Errorf("Cannot specify function version in spec on a created function (%d)", function.Spec.Version)
	}

	if function.Spec.Alias != "" {
		return errors.Errorf("Cannot specify alias on a created function (%s)", function.Spec.Alias)
	}

	return nil
}

func (c *Controller) handleFunctionCRUpdate(function *functioncr.Function) error {
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
