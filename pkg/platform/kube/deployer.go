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
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/nuclio-sdk"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type deployer struct {
	logger            nuclio.Logger
	deployOptions     *platform.DeployOptions
	kubeCommonOptions *CommonOptions
	consumer          *consumer
	platform          platform.Platform
}

func newDeployer(parentLogger nuclio.Logger, platform platform.Platform) (*deployer, error) {
	newdeployer := &deployer{
		logger:   parentLogger.GetChild("deployer").(nuclio.Logger),
		platform: platform,
	}

	return newdeployer, nil
}

func (d *deployer) deploy(consumer *consumer, deployOptions *platform.DeployOptions) (*platform.DeployResult, error) {
	var runResult *platform.DeployResult

	// save options, consumer
	d.deployOptions = deployOptions
	d.kubeCommonOptions = deployOptions.Common.Platform.(*CommonOptions)
	d.consumer = consumer

	d.logger.InfoWith("Running function", "name", deployOptions.Common.Identifier)

	// create a function, set default values and try to update from file
	functioncrInstance := functioncr.Function{}
	functioncrInstance.SetDefaults()
	functioncrInstance.Name = deployOptions.Common.Identifier

	if deployOptions.SpecPath != "" {
		err := functioncr.FromSpecFile(deployOptions.SpecPath, &functioncrInstance)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to read function spec file")
		}
	}

	// override with options
	if err := UpdateFunctioncrWithOptions(d.kubeCommonOptions, deployOptions, &functioncrInstance); err != nil {
		return nil, errors.Wrap(err, "Failed to update function with options")
	}

	if err := d.deletePreexistingFunction(d.kubeCommonOptions.Namespace, deployOptions.Common.Identifier); err != nil {
		return nil, errors.Wrap(err, "Failed to delete pre-existing function")
	}

	// ask the platform to do a build
	processorImageName, err := d.platform.BuildFunction(&deployOptions.Build)
	if err != nil {
		return nil, errors.Wrap(err, "Platform failed to build processor image")
	}

	// set the image
	functioncrInstance.Spec.Image = fmt.Sprintf("%s/%s", deployOptions.RunRegistry, processorImageName)

	// deploy the function
	err = d.deployFunction(&functioncrInstance)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to deploy function")
	}

	// get the function (might take a few seconds til it's created)
	service, err := d.getFunctionService(d.kubeCommonOptions.Namespace, deployOptions.Common.Identifier)
	if err == nil {
		runResult = &platform.DeployResult{
			Port: int(service.Spec.Ports[0].NodePort),
		}
	}

	d.logger.InfoWith("Function run complete", "node_port", runResult.Port)

	return runResult, nil
}

func UpdateFunctioncrWithOptions(kubeCommonOptions *CommonOptions,
	deployOptions *platform.DeployOptions,
	functioncrInstance *functioncr.Function) error {

	if deployOptions.Description != "" {
		functioncrInstance.Spec.Description = deployOptions.Description
	}

	// update replicas if scale was specified
	if deployOptions.Scale != "" {

		// TODO: handle/Set Min/Max replicas (used only with Auto mode)
		if deployOptions.Scale == "auto" {
			functioncrInstance.Spec.Replicas = 0
		} else {
			i, err := strconv.Atoi(deployOptions.Scale)
			if err != nil {
				return fmt.Errorf(`Invalid function scale, must be "auto" or an integer value`)
			}

			functioncrInstance.Spec.Replicas = int32(i)
		}
	}

	// Set specified labels, is label = "" remove it (if exists)
	labels := common.StringToStringMap(deployOptions.Labels)

	// create map if it doesn't exist and there are labels
	if len(labels) > 0 && functioncrInstance.Labels == nil {
		functioncrInstance.Labels = map[string]string{}
	}

	for labelName, labelValue := range labels {
		if labelName != "name" && labelName != "version" && labelName != "alias" {
			if labelValue == "" {
				delete(functioncrInstance.Labels, labelName)
			} else {
				functioncrInstance.Labels[labelName] = labelValue
			}
		}
	}

	envmap := common.StringToStringMap(deployOptions.Env)
	newenv := []v1.EnvVar{}

	// merge new Environment var: update existing then add new
	for _, e := range functioncrInstance.Spec.Env {
		if v, ok := envmap[e.Name]; ok {
			if v != "" {
				newenv = append(newenv, v1.EnvVar{Name: e.Name, Value: v})
			}
			delete(envmap, e.Name)
		} else {
			newenv = append(newenv, e)
		}
	}

	for k, v := range envmap {
		newenv = append(newenv, v1.EnvVar{Name: k, Value: v})
	}

	functioncrInstance.Spec.Env = newenv

	if deployOptions.HTTPPort != 0 {
		functioncrInstance.Spec.HTTPPort = deployOptions.HTTPPort
	}

	if deployOptions.Publish {
		functioncrInstance.Spec.Publish = deployOptions.Publish
	}

	if deployOptions.Disabled {
		functioncrInstance.Spec.Disabled = deployOptions.Disabled // TODO: use string to detect if noop/true/false
	}

	// update data bindings
	functioncrInstance.Spec.DataBindings = deployOptions.DataBindings

	// set namespace
	if kubeCommonOptions.Namespace != "" {
		functioncrInstance.Namespace = kubeCommonOptions.Namespace
	}

	return nil
}

func (d *deployer) deployFunction(functioncrToCreate *functioncr.Function) error {

	// get invocation logger. if it wasn't passed, use instance logger
	d.deployOptions.Common.GetLogger(d.logger).DebugWith("Deploying function", "function", functioncrToCreate)

	createdFunctioncr, err := d.consumer.functioncrClient.Create(functioncrToCreate)
	if err != nil {
		return err
	}

	// wait until function is processed
	return d.consumer.functioncrClient.WaitUntilCondition(createdFunctioncr.Namespace,
		createdFunctioncr.Name,
		functioncr.WaitConditionProcessed,
		10*time.Second,
	)
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

func (d *deployer) deletePreexistingFunction(namespace string, name string) error {

	// before we do anything, delete the current version of the function if it exists
	_, err := d.consumer.functioncrClient.Get(namespace, name)

	// note that existingFunctioncrInstance will contain a value regardless of whether there was an error
	if err != nil {

		// if it wasn't a not found error, log a warning
		if !apierrors.IsNotFound(err) {

			// don't fail, maybe we'll succeed in deploying
			d.logger.WarnWith("Failed to get function while checking if it already exists", "err", err)
		}

	} else {

		// if the function exists, delete it
		d.logger.InfoWith("Function already exists, deleting")

		if err := d.consumer.functioncrClient.Delete(namespace, name, &meta_v1.DeleteOptions{}); err != nil {

			// don't fail
			d.logger.WarnWith("Failed to delete existing function", "err", err)
		} else {

			// wait a bit to work around a controller bug
			time.Sleep(2 * time.Second)
		}
	}

	return nil
}
