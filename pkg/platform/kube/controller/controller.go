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
	"context"
	"os"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/platform/kube/apigatewayres"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/functionres"
	"github.com/nuclio/nuclio/pkg/platform/kube/monitoring"
	"github.com/nuclio/nuclio/pkg/platformconfig"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/v3io/version-go"
	"k8s.io/client-go/kubernetes"
)

type Controller struct {
	logger                    logger.Logger
	namespace                 string
	kubeClientSet             kubernetes.Interface
	nuclioClientSet           nuclioioclient.Interface
	functionresClient         functionres.Client
	apigatewayresClient       apigatewayres.Client
	imagePullSecrets          string
	platformConfiguration     *platformconfig.Config
	platformConfigurationName string
	externalIPAddresses       []string

	// (re)syncers
	functionOperator      *functionOperator
	projectOperator       *projectOperator
	functionEventOperator *functionEventOperator
	apiGatewayOperator    *apiGatewayOperator
	resyncInterval        time.Duration

	// monitors
	cronJobMonitoring          *CronJobMonitoring
	functionMonitoring         *monitoring.FunctionMonitor
	functionMonitoringInterval time.Duration
}

func NewController(parentLogger logger.Logger,
	namespace string,
	imagePullSecrets string,
	kubeClientSet kubernetes.Interface,
	nuclioClientSet nuclioioclient.Interface,
	functionresClient functionres.Client,
	apigatewayresClient apigatewayres.Client,
	resyncInterval time.Duration,
	functionMonitoringInterval time.Duration,
	cronJobStaleResourcesCleanupInterval time.Duration,
	platformConfiguration *platformconfig.Config,
	platformConfigurationName string,
	functionOperatorNumWorkers int,
	functionEventOperatorNumWorkers int,
	projectOperatorNumWorkers int,
	apiGatewayOperatorNumWorkers int) (*Controller, error) {
	var err error

	// replace "*" with "", which is actually "all" in kube-speak
	if namespace == "*" {
		namespace = ""
	}

	ctx := context.Background()

	newController := &Controller{
		logger:                     parentLogger,
		namespace:                  namespace,
		imagePullSecrets:           imagePullSecrets,
		kubeClientSet:              kubeClientSet,
		nuclioClientSet:            nuclioClientSet,
		functionresClient:          functionresClient,
		apigatewayresClient:        apigatewayresClient,
		platformConfiguration:      platformConfiguration,
		platformConfigurationName:  platformConfigurationName,
		resyncInterval:             resyncInterval,
		functionMonitoringInterval: functionMonitoringInterval,
	}

	newController.logger.DebugWithCtx(ctx, "Read configuration",
		"platformConfig", newController.platformConfiguration,
		"version", version.Get())

	// set ourselves as the platform configuration provider of the function resource client (it needs it to do
	// stuff when creating stuff)
	functionresClient.SetPlatformConfigurationProvider(newController)

	// create a function operator
	newController.functionOperator, err = newFunctionOperator(ctx, parentLogger,
		newController,
		&newController.resyncInterval,
		imagePullSecrets,
		functionresClient,
		functionOperatorNumWorkers)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create functions operator")
	}

	// create a function event operator
	newController.functionEventOperator, err = newFunctionEventOperator(ctx,
		parentLogger,
		newController,
		&newController.resyncInterval,
		functionEventOperatorNumWorkers)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function event operator")
	}

	// create a project operator
	newController.projectOperator, err = newProjectOperator(ctx,
		parentLogger,
		newController,
		&newController.resyncInterval,
		projectOperatorNumWorkers)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create project operator")
	}

	// create an api gateway operator
	newController.apiGatewayOperator, err = newAPIGatewayOperator(ctx,
		parentLogger,
		newController,
		&newController.resyncInterval,
		apiGatewayOperatorNumWorkers)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create api gateway operator")
	}

	newController.functionMonitoring, err = monitoring.NewFunctionMonitor(ctx,
		parentLogger,
		namespace,
		kubeClientSet,
		nuclioClientSet,
		functionMonitoringInterval)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function monitor")
	}

	// create cron job monitoring
	if platformConfiguration.CronTriggerCreationMode == platformconfig.KubeCronTriggerCreationMode {
		newController.cronJobMonitoring = NewCronJobMonitoring(ctx,
			parentLogger,
			newController,
			&cronJobStaleResourcesCleanupInterval)
	}

	return newController, nil
}

func (c *Controller) Start(ctx context.Context) error {
	c.logger.InfoWithCtx(ctx,
		"Starting controller",
		"namespace", c.namespace)

	// start operators
	if err := c.startOperators(ctx); err != nil {
		return errors.Wrap(err, "Failed to start operators")
	}

	// start monitors
	if err := c.startMonitors(ctx); err != nil {
		return errors.Wrap(err, "Failed to start monitors")
	}

	c.logger.InfoWithCtx(ctx, "Controller has successfully started", "namespace", c.namespace)
	return nil
}

func (c *Controller) Stop(ctx context.Context) error {
	// TODO: stop operators

	// stop cronjob monitoring
	if c.cronJobMonitoring != nil {
		c.cronJobMonitoring.stop(ctx)
	}

	// stop function monitor
	c.functionMonitoring.Stop(ctx)
	return nil
}

func (c *Controller) GetPlatformConfiguration() *platformconfig.Config {
	return c.platformConfiguration
}

func (c *Controller) GetExternalIPAddresses() []string {
	if len(c.externalIPAddresses) > 0 {
		return c.externalIPAddresses
	}

	c.externalIPAddresses = strings.Split(os.Getenv("NUCLIO_CONTROLLER_EXTERNAL_IP_ADDRESSES"), ",")
	return c.externalIPAddresses
}

func (c *Controller) GetPlatformConfigurationName() string {
	return c.platformConfigurationName
}

func (c *Controller) GetResyncInterval() time.Duration {
	return c.resyncInterval
}

func (c *Controller) GetFunctionMonitoringInterval() time.Duration {
	return c.functionMonitoringInterval
}

func (c *Controller) SetFunctionMonitoringInterval(interval time.Duration) {
	c.functionMonitoringInterval = interval
}

func (c *Controller) GetFunctionMonitoring() *monitoring.FunctionMonitor {
	return c.functionMonitoring
}

func (c *Controller) startOperators(ctx context.Context) error {

	// start the function operator
	if err := c.functionOperator.start(ctx); err != nil {
		return errors.Wrap(err, "Failed to start function operator")
	}

	// start the project operator
	if err := c.projectOperator.start(ctx); err != nil {
		return errors.Wrap(err, "Failed to start project operator")
	}

	// start the function event operator
	if err := c.functionEventOperator.start(ctx); err != nil {
		return errors.Wrap(err, "Failed to start function event operator")
	}

	// start the api gateway operator
	if err := c.apiGatewayOperator.start(ctx); err != nil {
		return errors.Wrap(err, "Failed to start api gateway operator")
	}

	return nil
}

func (c *Controller) startMonitors(ctx context.Context) error {

	// start function monitor
	if err := c.functionMonitoring.Start(ctx); err != nil {
		return errors.Wrap(err, "Failed to start function monitor")
	}

	if c.cronJobMonitoring != nil {

		// start cron job monitoring
		c.cronJobMonitoring.start(ctx)
	}

	return nil
}
