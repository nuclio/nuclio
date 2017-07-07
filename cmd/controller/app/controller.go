package app

import (
	"github.com/nuclio/nuclio/pkg/k8s/customresource/function"
	"github.com/nuclio/nuclio/pkg/logger"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	//apierrors "k8s.io/apimachinery/pkg/api/errors"
	//"k8s.io/apimachinery/pkg/runtime/serializer"
	//meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//v1b1e "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	//"k8s.io/client-go/pkg/api/v1"
	//"k8s.io/apimachinery/pkg/runtime"
	//"k8s.io/apimachinery/pkg/runtime/schema"
	//"k8s.io/apimachinery/pkg/util/wait"

	"github.com/nuclio/nuclio-zap"
	"github.com/pkg/errors"
)

type Controller struct {
	logger                 logger.Logger
	restConfig             *rest.Config
	clientSet              *kubernetes.Clientset
	functionCustomResource *function.CustomResource
	// controlMessageChan chan controlMessage
}

func NewController(configurationPath string) (*Controller, error) {
	var err error

	newController := &Controller{}

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

	newController.functionCustomResource, err = function.NewCustomResource(newController.logger,
		newController.restConfig,
		newController.clientSet)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create custom resource object")
	}

	// ensure that the "functions" third party resource exists in kubernetes
	err = newController.functionCustomResource.CreateResource()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create custom resource object")
	}

	return nil, nil
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
