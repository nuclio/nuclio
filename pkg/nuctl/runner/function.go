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
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/nuctl/builder"
	"github.com/nuclio/nuclio/pkg/util/common"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
)

type FunctionRunner struct {
	nucliocli.KubeConsumer
	logger  nuclio.Logger
	options *Options
}

func NewFunctionRunner(parentLogger nuclio.Logger, options *Options) (*FunctionRunner, error) {
	var err error

	newFunctionRunner := &FunctionRunner{
		logger:  parentLogger.GetChild("runner").(nuclio.Logger),
		options: options,
	}

	// get kube stuff
	_, err = newFunctionRunner.GetClients(newFunctionRunner.logger, options.Common.KubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get clients")
	}

	return newFunctionRunner, nil
}

func (fr *FunctionRunner) Execute() error {
	fr.logger.InfoWith("Running function", "name", fr.options.Common.Identifier)

	// create a function, set default values and try to update from file
	functioncrInstance := functioncr.Function{}
	functioncrInstance.SetDefaults()
	functioncrInstance.Name = fr.options.Common.Identifier

	if fr.options.SpecPath != "" {
		err := functioncr.FromSpecFile(fr.options.SpecPath, &functioncrInstance)
		if err != nil {
			return errors.Wrap(err, "Failed to read function spec file")
		}
	}

	// override with options
	if err := UpdateFunctioncrWithOptions(fr.options, &functioncrInstance); err != nil {
		return errors.Wrap(err, "Failed to update function with options")
	}

	// create a builder
	builder, err := builder.NewFunctionBuilder(fr.logger, &fr.options.Build)
	if err != nil {
		return errors.Wrap(err, "Failed to create builder")
	}

	// execute the build
	err = builder.Execute()
	if err != nil {
		return err
	}

	// deploy the function
	err = fr.deployFunction(&functioncrInstance)
	if err != nil {
		return errors.Wrap(err, "Failed to deploy function")
	}

	fr.logger.Info("Function run complete")

	return nil
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
			} else {
				functioncrInstance.Spec.Replicas = int32(i)
			}
		}
	}

	// Set specified labels, is label = "" remove it (if exists)
	labels := common.StringToStringMap(options.Labels)
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

	// if the user passed image in command line arguments
	if options.Build.ImageName != "" {

		// use that no matter what
		functioncrInstance.Spec.Image = options.Build.ImageName

	// if the user *didn't* pass image in command line arguments and image wasn't specified in
	// the spec file, use a default for now (assuming registry proxy)
	} else if functioncrInstance.Spec.Image == "" {

		functioncrInstance.Spec.Image = fmt.Sprintf("localhost:5000/%s:%s", options.Common.Identifier, "latest")
	}

	// update data bindings
	if err := updateDataBindings(options.DataBindings, functioncrInstance); err != nil {
		return errors.Wrap(err, "Failed to decode data bindings")
	}

	return nil
}

func updateDataBindings(encodedDataBindings string, function *functioncr.Function) error {

	// if user passed nothing, no data bindings required
	if encodedDataBindings == "" {
		return nil
	}

	return json.Unmarshal([]byte(encodedDataBindings), &function.Spec.DataBindings)
}

func (fr *FunctionRunner) deployFunction(functioncrToCreate *functioncr.Function) error {
	createdFunctioncr, err := fr.FunctioncrClient.Create(functioncrToCreate)
	if err != nil {
		return err
	}

	// wait until function is processed
	return fr.FunctioncrClient.WaitUntilCondition(createdFunctioncr.Namespace,
		createdFunctioncr.Name,
		functioncr.WaitConditionProcessed,
		10*time.Second,
	)
}
