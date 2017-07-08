package app

import (
	"fmt"

	"github.com/nuclio/nuclio-zap"
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/functiondep"
	"github.com/nuclio/nuclio/pkg/logger"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Controller struct {
	logger                logger.Logger
	restConfig            *rest.Config
	clientSet             *kubernetes.Clientset
	functioncrClient      *functioncr.Client
	functioncrChangesChan chan functioncr.Change
	functiondepClient     *functiondep.Client
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

	// wait for changes on the function custom resource
	c.functioncrClient.WatchForChanges(c.functioncrChangesChan)

	for {
		functionChange := <-c.functioncrChangesChan

		switch functionChange.Kind {
		case functioncr.ChangeKindAdded:
			err = c.handleCustomResourceAddOrUpdate(functionChange.Function)
		default:
			err = fmt.Errorf("Unknown change kind: %d", functionChange.Kind)
		}

		if err != nil {
			return errors.Wrap(err, "Failed to handle function custom resource change")
		}
	}
}

func (c *Controller) getClientConfig(configurationPath string) (*rest.Config, error) {
	if configurationPath != "" {
		return clientcmd.BuildConfigFromFlags("", configurationPath)
	}

	return rest.InClusterConfig()
}

func (c *Controller) createLogger() (logger.Logger, error) {

	// TODO: configuration stuff
	return nucliozap.NewNuclioZap("controller")
}

func (c *Controller) handleCustomResourceAddOrUpdate(function *functioncr.Function) error {
	c.logger.DebugWith("Function custom resource added/updated",
		"gen", function.ResourceVersion,
		"namespace", function.Namespace)

	// try to get this deployment
	deployment, err := c.functiondepClient.Get(function.Namespace, function.Name)
	if err != nil {
		return errors.Wrap(err, "Failed to get function deployment")
	}

	// if the deployment doesn't exist, we need to create it
	if deployment == nil {
		c.logger.Debug("Deployment doesn't exist, creating")

		_, err := c.functiondepClient.CreateOrUpdate(function)
		if err != nil {
			return errors.Wrap(err, "Failed to create deployment")
		}
	}

	return nil
}
