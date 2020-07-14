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
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/loggersink"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/controller"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	// load all sinks
	_ "github.com/nuclio/nuclio/pkg/sinks"

	"github.com/nuclio/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func Run(kubeconfigPath string,
	namespace string,
	imagePullSecrets string,
	platformConfigurationPath string,
	functionOperatorNumWorkersStr string,
	functionOperatorResyncIntervalStr string,
	cronJobStaleResourcesCleanupIntervalStr string,
	functionEventOperatorNumWorkersStr string,
	projectOperatorNumWorkersStr string) error {

	newController, err := createController(kubeconfigPath,
		namespace,
		imagePullSecrets,
		platformConfigurationPath,
		functionOperatorNumWorkersStr,
		functionOperatorResyncIntervalStr,
		cronJobStaleResourcesCleanupIntervalStr,
		functionEventOperatorNumWorkersStr,
		projectOperatorNumWorkersStr)
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
	namespace string,
	imagePullSecrets string,
	platformConfigurationPath string,
	functionOperatorNumWorkersStr string,
	functionOperatorResyncIntervalStr string,
	cronJobStaleResourcesCleanupIntervalStr string,
	functionEventOperatorNumWorkersStr string,
	projectOperatorNumWorkersStr string) (*controller.Controller, error) {

	functionOperatorNumWorkers, err := strconv.Atoi(functionOperatorNumWorkersStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve number of workers for function operator")
	}

	functionEventOperatorNumWorkers, err := strconv.Atoi(functionEventOperatorNumWorkersStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve number of workers for function event operator")
	}

	functionOperatorResyncInterval, err := time.ParseDuration(functionOperatorResyncIntervalStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse resync interval for function operator")
	}

	cronJobStaleResourcesCleanupInterval, err := time.ParseDuration(cronJobStaleResourcesCleanupIntervalStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse cron job stale pods deletion interval")
	}

	projectOperatorNumWorkers, err := strconv.Atoi(projectOperatorNumWorkersStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve number of workers for project operator")
	}

	// get platform configuration
	platformConfiguration, err := platformconfig.NewPlatformConfig(platformConfigurationPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get platform configuration")
	}

	// create a root logger
	rootLogger, err := loggersink.CreateSystemLogger("controller", platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create logger")
	}

	restConfig, err := getClientConfig(kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get client configuration")
	}

	kubeClientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create k8s client set")
	}

	nuclioClientSet, err := nuclioioclient.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create nuclio client set")
	}

	// create a client for function deployments
	functionresClient, err := functionres.NewLazyClient(rootLogger, kubeClientSet, nuclioClientSet)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function deployment client")
	}

	newController, err := controller.NewController(rootLogger,
		namespace,
		imagePullSecrets,
		kubeClientSet,
		nuclioClientSet,
		functionresClient,
		functionOperatorResyncInterval,
		cronJobStaleResourcesCleanupInterval,
		platformConfiguration,
		functionOperatorNumWorkers,
		functionEventOperatorNumWorkers,
		projectOperatorNumWorkers)

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
