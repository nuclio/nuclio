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

package kube

import (
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"

	"github.com/nuclio/logger"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type deployer struct {
	logger        logger.Logger
	consumer      *consumer
	platform      *Platform
}

func newDeployer(parentLogger logger.Logger, consumer *consumer, platform *Platform) (*deployer, error) {
	newdeployer := &deployer{
		logger:   parentLogger.GetChild("deployer"),
		platform: platform,
		consumer: consumer,
	}

	return newdeployer, nil
}

func (d *deployer) createOrUpdateFunctioncr(functioncrInstance *functioncr.Function,
	deployOptions *platform.DeployOptions,
	functionStatus *functionconfig.Status) (*functioncr.Function, error) {

	var err error

	// boolean which indicates whether the function existed or not
	functionExisted := functioncrInstance != nil

	deployOptions.Logger.DebugWith("Creating/updating functioncr",
		"existed", functionExisted)

	if functioncrInstance == nil {
		functioncrInstance = &functioncr.Function{}
		functioncrInstance.SetDefaults()
	}

	// convert config, status -> functioncr
	d.populateFunctioncr(&deployOptions.FunctionConfig, functionStatus, functioncrInstance)

	deployOptions.Logger.DebugWith("Populated functioncr with configuration and status",
		"functioncr", functioncrInstance)

	// if function didn't exist, create. otherwise update
	if !functionExisted {
		functioncrInstance, err = d.consumer.functioncrClient.Create(functioncrInstance)
	} else {
		functioncrInstance, err = d.consumer.functioncrClient.Update(functioncrInstance)
	}

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create/update functioncr")
	}

	return functioncrInstance, nil
}

func (d *deployer) populateFunctioncr(functionConfig *functionconfig.Config,
	functionStatus *functionconfig.Status,
	functioncrInstance *functioncr.Function) {

	functioncrInstance.Spec = functionConfig.Spec

	// set meta
	functioncrInstance.Name = functionConfig.Meta.Name
	functioncrInstance.Namespace = functionConfig.Meta.Namespace
	functioncrInstance.Labels = functionConfig.Meta.Labels
	functioncrInstance.Annotations = functionConfig.Meta.Annotations

	// set alias as "latest" for now
	functioncrInstance.Spec.Alias = "latest"

	functioncrInstance.Spec.ImageName = fmt.Sprintf("%s/%s",
		functionConfig.Spec.RunRegistry,
		functionConfig.Spec.ImageName)

	// update status
	functioncrInstance.Status.Status = *functionStatus
}

func (d *deployer) deploy(functioncrInstance *functioncr.Function,
	deployOptions *platform.DeployOptions) (*platform.DeployResult, error) {

	// get the logger with which we need to deploy
	deployLogger := deployOptions.Logger
	if deployLogger == nil {
		deployLogger = d.logger
	}

	// do the create / update
	d.createOrUpdateFunctioncr(functioncrInstance,
		deployOptions,
		&functionconfig.Status{
			State: functionconfig.FunctionStateNotReady,
		})

	// wait for the function to be ready
	err := d.waitForFunctionReadiness(deployLogger, functioncrInstance, deployOptions.ReadinessTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to wait for function readiness")
	}

	// get the function service (might take a few seconds til it's created)
	service, err := d.getFunctionService(deployOptions.FunctionConfig.Meta.Namespace,
		deployOptions.FunctionConfig.Meta.Name)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get function service")
	}

	return &platform.DeployResult{
		Port: int(service.Spec.Ports[0].NodePort),
	}, nil
}

func (d *deployer) waitForFunctionReadiness(deployLogger logger.Logger,
	functioncrInstance *functioncr.Function,
	timeout *time.Duration) error {

	// TODO: you can't log a nil pointer without panicing - maybe this should be a logger-wide behavior
	var logReadinessTimeout interface{}
	if timeout == nil {
		logReadinessTimeout = "nil"
	} else {
		logReadinessTimeout = timeout
	}

	deployLogger.InfoWith("Waiting for function to be ready", "timeout", logReadinessTimeout)

	// wait until function is ready
	err := d.consumer.functioncrClient.WaitUntilCondition(functioncrInstance.Namespace,
		functioncrInstance.Name,
		functioncr.WaitConditionReady,
		timeout,
	)

	if err != nil {
		return errors.Wrap(err, "Function wasn't ready in time")
	}

	return nil
}

func (d *deployer) getFunctionService(namespace string, name string) (service *v1.Service, err error) {
	deadline := time.Now().Add(10 * time.Second)

	for {

		// after a few seconds, give up
		if time.Now().After(deadline) {
			break
		}

		service, err = d.consumer.clientset.CoreV1().Services(namespace).Get(name, meta_v1.GetOptions{})

		// if there was an error other than the fact that the service wasn't found,
		// return now
		if !apierrors.IsNotFound(err) {
			return
		}

		// if we got a service, check that it has a node port
		if service != nil && len(service.Spec.Ports) > 0 && service.Spec.Ports[0].NodePort != 0 {
			return
		}

		// wait a bit
		time.Sleep(1 * time.Second)
	}

	return
}
