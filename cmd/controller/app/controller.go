/*
Copyright 2023 The Nuclio Authors.

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
	"context"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/loggersink"
	"github.com/nuclio/nuclio/pkg/platform/kube/apigatewayres"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/controller"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	// load all sinks
	_ "github.com/nuclio/nuclio/pkg/sinks"
)

func Run(kubeconfigPath string,
	namespace string,
	imagePullSecrets string,
	platformConfigurationPath string,
	platformConfigurationName string,
	functionOperatorNumWorkersStr string,
	resyncIntervalStr string,
	functionMonitorIntervalStr string,
	scalingGracePeriodStr string,
	cronJobStaleResourcesCleanupIntervalStr string,
	evictedPodsCleanupIntervalStr string,
	functionEventOperatorNumWorkersStr string,
	projectOperatorNumWorkersStr string,
	apiGatewayOperatorNumWorkersStr string) error {

	newController, err := createController(kubeconfigPath,
		namespace,
		imagePullSecrets,
		platformConfigurationPath,
		platformConfigurationName,
		functionOperatorNumWorkersStr,
		resyncIntervalStr,
		functionMonitorIntervalStr,
		scalingGracePeriodStr,
		cronJobStaleResourcesCleanupIntervalStr,
		evictedPodsCleanupIntervalStr,
		functionEventOperatorNumWorkersStr,
		projectOperatorNumWorkersStr,
		apiGatewayOperatorNumWorkersStr)
	if err != nil {
		return errors.Wrap(err, "Failed to create controller")
	}

	// start the controller
	if err := newController.Start(context.Background()); err != nil {
		return errors.Wrap(err, "Failed to start controller")
	}

	// TODO: stop
	select {}
}

func createController(kubeconfigPath string,
	namespace string,
	imagePullSecrets string,
	platformConfigurationPath string,
	platformConfigurationName string,
	functionOperatorNumWorkersStr string,
	resyncIntervalStr string,
	functionMonitorIntervalStr string,
	scalingGracePeriodStr string,
	cronJobStaleResourcesCleanupIntervalStr string,
	evictedPodsCleanupIntervalStr string,
	functionEventOperatorNumWorkersStr string,
	projectOperatorNumWorkersStr string,
	apiGatewayOperatorNumWorkersStr string) (*controller.Controller, error) {

	functionOperatorNumWorkers, err := strconv.Atoi(functionOperatorNumWorkersStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve number of workers for function operator")
	}

	functionEventOperatorNumWorkers, err := strconv.Atoi(functionEventOperatorNumWorkersStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve number of workers for function event operator")
	}

	resyncInterval, err := time.ParseDuration(resyncIntervalStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse resync interval for function operator")
	}

	functionMonitorInterval, err := time.ParseDuration(functionMonitorIntervalStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse function monitor interval")
	}

	scalingGracePeriod, err := time.ParseDuration(scalingGracePeriodStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse function scaling grace period")
	}

	cronJobStaleResourcesCleanupInterval, err := time.ParseDuration(cronJobStaleResourcesCleanupIntervalStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse cron job stale pods deletion interval")
	}

	evictedPodsCleanupInterval, err := time.ParseDuration(evictedPodsCleanupIntervalStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse cron job stale pods deletion interval")
	}

	projectOperatorNumWorkers, err := strconv.Atoi(projectOperatorNumWorkersStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve number of workers for project operator")
	}

	apiGatewayOperatorNumWorkers, err := strconv.Atoi(apiGatewayOperatorNumWorkersStr)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve number of workers for api gateway operator")
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

	restConfig, err := common.GetClientConfig(kubeconfigPath)
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

	// create cmd runner
	cmdRunner, err := cmdrunner.NewShellRunner(rootLogger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create cmd runner")
	}

	// create ingress manager
	ingressManager, err := ingress.NewManager(rootLogger, kubeClientSet, cmdRunner, platformConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create ingress manager")
	}

	// create api gateway provisioner
	apigatewayresClient, err := apigatewayres.NewLazyClient(rootLogger, kubeClientSet, nuclioClientSet, ingressManager)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create api gateway provisioner")
	}

	rest.SetDefaultWarningHandler(common.NewKubernetesClientWarningHandler(rootLogger.GetChild("kube_warnings")))

	newController, err := controller.NewController(rootLogger,
		namespace,
		imagePullSecrets,
		kubeClientSet,
		nuclioClientSet,
		functionresClient,
		apigatewayresClient,
		resyncInterval,
		functionMonitorInterval,
		scalingGracePeriod,
		cronJobStaleResourcesCleanupInterval,
		evictedPodsCleanupInterval,
		platformConfiguration,
		platformConfigurationName,
		functionOperatorNumWorkers,
		functionEventOperatorNumWorkers,
		projectOperatorNumWorkers,
		apiGatewayOperatorNumWorkers)

	if err != nil {
		return nil, err
	}

	return newController, nil
}
