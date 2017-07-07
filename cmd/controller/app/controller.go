package app

import (
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/logger"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/nuclio/nuclio-zap"
	"github.com/pkg/errors"
)

type Controller struct {
	logger                 logger.Logger
	restConfig             *rest.Config
	clientSet              *kubernetes.Clientset
	functioncrClient *functioncr.Client
	functioncrChangesChan    chan functioncr.Change
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

	return newController, nil
}

func (c *Controller) Start() error {
	var err error

	c.functioncrClient, err = functioncr.NewClient(c.logger,
		c.restConfig,
		c.clientSet)

	if err != nil {
		return errors.Wrap(err, "Failed to create custom resource object")
	}

	// ensure that the "functions" third party resource exists in kubernetes
	err = c.functioncrClient.CreateResource()
	if err != nil {
		return errors.Wrap(err, "Failed to create custom resource object")
	}

	// wait for changes on the function custom resource
	c.functioncrClient.WatchForChanges(c.functioncrChangesChan)

	for {
		functionChange := <-c.functioncrChangesChan
		c.logger.DebugWith("Got update",
			"kind", functionChange.Kind,
			"gen", functionChange.Function.ResourceVersion)
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
