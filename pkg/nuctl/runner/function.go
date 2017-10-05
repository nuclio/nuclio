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

package runner

import (
	"fmt"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/nuctl/builder"

	"github.com/nuclio/nuclio-sdk"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionRunner struct {
	logger       nuclio.Logger
	options      *Options
	kubeConsumer *nuctl.KubeConsumer
}

type RunResult struct {
	NodePort int
}

func NewFunctionRunner(parentLogger nuclio.Logger) (*FunctionRunner, error) {
	newFunctionRunner := &FunctionRunner{
		logger: parentLogger.GetChild("runner").(nuclio.Logger),
	}

	return newFunctionRunner, nil
}

func (fr *FunctionRunner) Run(kubeConsumer *nuctl.KubeConsumer, options *Options) (*RunResult, error) {
	var runResult *RunResult

	// save options, consumer
	fr.options = options
	fr.kubeConsumer = kubeConsumer

	fr.logger.InfoWith("Running function", "name", options.Common.Identifier)

	// create a function, set default values and try to update from file
	functioncrInstance := functioncr.Function{}
	functioncrInstance.SetDefaults()
	functioncrInstance.Name = options.Common.Identifier

	if options.SpecPath != "" {
		err := functioncr.FromSpecFile(options.SpecPath, &functioncrInstance)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to read function spec file")
		}
	}

	// override with options
	if err := UpdateFunctioncrWithOptions(options, &functioncrInstance); err != nil {
		return nil, errors.Wrap(err, "Failed to update function with options")
	}

	if err := fr.deletePreexistingFunction(options.Common.Namespace, options.Common.Identifier); err != nil {
		return nil, errors.Wrap(err, "Failed to delete pre-existing function")
	}

	// create a builder
	builder, err := builder.NewFunctionBuilder(fr.logger)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create builder")
	}

	// execute the build, set image
	processorImageName, err := builder.Build(&options.Build)
	if err != nil {
		return nil, err
	}

	// set the image
	functioncrInstance.Spec.Image = fmt.Sprintf("%s/%s", options.RunRegistry, processorImageName)

	// deploy the function
	err = fr.deployFunction(&functioncrInstance)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to deploy function")
	}

	// get the function (might take a few seconds til it's created)
	service, err := fr.getFunctionService(options.Common.Namespace, options.Common.Identifier)
	if err == nil {
		runResult = &RunResult{
			NodePort: int(service.Spec.Ports[0].NodePort),
		}
	}

	fr.logger.InfoWith("Function run complete", "node_port", runResult.NodePort)

	return runResult, nil
}

func UpdateFunctioncrWithOptions(options *Options, functioncrInstance *functioncr.Function) error {

	if options.Description != "" {
		functioncrInstance.Spec.Description = options.Description
	}

	// update replicas if scale was specified
	if options.Scale != "" {

		// TODO: handle/Set Min/Max replicas (used only with Auto mode)
		if options.Scale == "auto" {
			functioncrInstance.Spec.Replicas = 0
		} else {
			i, err := strconv.Atoi(options.Scale)
			if err != nil {
				return fmt.Errorf(`Invalid function scale, must be "auto" or an integer value`)
			}

			functioncrInstance.Spec.Replicas = int32(i)
		}
	}

	// Set specified labels, is label = "" remove it (if exists)
	labels := common.StringToStringMap(options.Labels)

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

	envmap := common.StringToStringMap(options.Env)
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

	if options.HTTPPort != 0 {
		functioncrInstance.Spec.HTTPPort = options.HTTPPort
	}

	if options.Publish {
		functioncrInstance.Spec.Publish = options.Publish
	}

	if options.Disabled {
		functioncrInstance.Spec.Disabled = options.Disabled // TODO: use string to detect if noop/true/false
	}

	// update data bindings
	functioncrInstance.Spec.DataBindings = options.DataBindings

	if options.Common.Namespace != "" {
		functioncrInstance.Namespace = options.Common.Namespace
	}

	return nil
}

func (fr *FunctionRunner) deployFunction(functioncrToCreate *functioncr.Function) error {

	// get invocation logger. if it wasn't passed, use instance logger
	fr.options.Common.GetLogger(fr.logger).DebugWith("Deploying function", "function", functioncrToCreate)

	createdFunctioncr, err := fr.kubeConsumer.FunctioncrClient.Create(functioncrToCreate)
	if err != nil {
		return err
	}

	// wait until function is processed
	return fr.kubeConsumer.FunctioncrClient.WaitUntilCondition(createdFunctioncr.Namespace,
		createdFunctioncr.Name,
		functioncr.WaitConditionProcessed,
		10*time.Second,
	)
}

func (fr *FunctionRunner) getFunctionService(namespace string, name string) (service *v1.Service, err error) {
	deadline := time.Now().Add(10 * time.Second)

	for {

		// after a few seconds, give up
		if time.Now().After(deadline) {
			break
		}

		service, err = fr.kubeConsumer.Clientset.CoreV1().Services(namespace).Get(name, meta_v1.GetOptions{})

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

func (fr *FunctionRunner) deletePreexistingFunction(namespace string, name string) error {

	// before we do anything, delete the current version of the function if it exists
	_, err := fr.kubeConsumer.FunctioncrClient.Get(namespace, name)

	// note that existingFunctioncrInstance will contain a value regardless of whether there was an error
	if err != nil {

		// if it wasn't a not found error, log a warning
		if !apierrors.IsNotFound(err) {

			// don't fail, maybe we'll succeed in deploying
			fr.logger.WarnWith("Failed to get function while checking if it already exists", "err", err)
		}

	} else {

		// if the function exists, delete it
		fr.logger.InfoWith("Function already exists, deleting")

		if err := fr.kubeConsumer.FunctioncrClient.Delete(namespace, name, &meta_v1.DeleteOptions{}); err != nil {

			// don't fail
			fr.logger.WarnWith("Failed to delete existing function", "err", err)
		} else {

			// wait a bit to work around a controller bug
			time.Sleep(2 * time.Second)
		}
	}

	return nil
}
