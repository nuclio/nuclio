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
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/controller"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func Run(kubeconfigPath string,
	resolvedNamespace string,
	imagePullSecrets string) error {

	newController, err := createController(kubeconfigPath, resolvedNamespace, imagePullSecrets)
	if err != nil {
		return errors.Wrap(err, "Failed to create controller")
	}

	// start the controller
	if err := newController.Start(); err != nil {
		return errors.Wrap(err, "Failed to start controller")
	}

	// TODO: stop
	select {}
}

func createController(kubeconfigPath string,
	resolvedNamespace string,
	imagePullSecrets string) (*controller.Controller, error) {

	// create a root logger
	rootLogger, err := createLogger()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create root logger")
	}

	restConfig, err := getClientConfig(kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get client configuration")
	}

	kubeClientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create k8s client set")
	}

	nuclioClientSet, err := nuclioio_client.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create nuclio client set")
	}

	// create a client for function deployments
	functionresClient, err := functionres.NewLazyClient(rootLogger, kubeClientSet)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function deployment client")
	}

	newController, err := controller.NewController(rootLogger,
		resolvedNamespace,
		imagePullSecrets,
		kubeClientSet,
		nuclioClientSet,
		functionresClient,
		5*time.Minute)

	if err != nil {
		return nil, err
	}

	return newController, nil
}

func getClientConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	return rest.InClusterConfig()
}

func createLogger() (logger.Logger, error) {
	return nucliozap.NewNuclioZapCmd("controller", nucliozap.DebugLevel)
}
