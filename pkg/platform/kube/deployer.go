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

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"

	"github.com/nuclio/nuclio-sdk"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type deployer struct {
	logger        nuclio.Logger
	deployOptions *platform.DeployOptions
	consumer      *consumer
	platform      platform.Platform
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
	d.consumer = consumer

	// create a function, set default values and try to update from file
	functioncrInstance := functioncr.Function{}
	functioncrInstance.SetDefaults()
	functioncrInstance.Name = deployOptions.Common.Identifier

	// override with options
	if err := UpdateFunctioncrWithOptions(deployOptions,
		&functioncrInstance); err != nil {
		return nil, errors.Wrap(err, "Failed to update function with options")
	}

	// set the image
	functioncrInstance.Spec.Image = fmt.Sprintf("%s/%s",
		deployOptions.RunRegistry,
		deployOptions.ImageName)

	// deploy the function
	err := d.deployFunction(&functioncrInstance)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to deploy function")
	}

	// get the function (might take a few seconds til it's created)
	service, err := d.getFunctionService(d.deployOptions.Common.Namespace, deployOptions.Common.Identifier)
	if err == nil {
		runResult = &platform.DeployResult{
			Port: int(service.Spec.Ports[0].NodePort),
		}
	}

	return runResult, nil
}

func UpdateFunctioncrWithOptions(deployOptions *platform.DeployOptions,
	functioncrInstance *functioncr.Function) error {

	functioncrInstance.Spec.Runtime = deployOptions.Build.Runtime
	functioncrInstance.Spec.Handler = deployOptions.Build.Handler
	functioncrInstance.Spec.Description = deployOptions.Description

	// update replicas if scale was specified
	if deployOptions.MinReplicas != 0 || deployOptions.MaxReplicas != 0 {
		functioncrInstance.Spec.Replicas = 0
	} else {
		functioncrInstance.Spec.Replicas = int32(deployOptions.Replicas)
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
		functioncrInstance.Spec.HTTPPort = int32(deployOptions.HTTPPort)
	}

	if deployOptions.Publish {
		functioncrInstance.Spec.Publish = deployOptions.Publish
	}

	if deployOptions.Disabled {
		functioncrInstance.Spec.Disabled = deployOptions.Disabled // TODO: use string to detect if noop/true/false
	}

	// update data bindings
	functioncrInstance.Spec.DataBindings = deployOptions.DataBindings

	// update triggers
	functioncrInstance.Spec.Triggers = deployOptions.Triggers

	// set namespace
	if deployOptions.Common.Namespace != "" {
		functioncrInstance.Namespace = deployOptions.Common.Namespace
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
