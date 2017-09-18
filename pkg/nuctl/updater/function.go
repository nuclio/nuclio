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

package updater

import (
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/nuctl/runner"

	"github.com/nuclio/nuclio-sdk"
)

type FunctionUpdater struct {
	logger       nuclio.Logger
	options      *Options
	kubeConsumer *nuctl.KubeConsumer
}

func NewFunctionUpdater(parentLogger nuclio.Logger) (*FunctionUpdater, error) {
	newFunctionUpdater := &FunctionUpdater{
		logger: parentLogger.GetChild("updater").(nuclio.Logger),
	}

	return newFunctionUpdater, nil
}

func (fu *FunctionUpdater) Execute(kubeConsumer *nuctl.KubeConsumer, options *Options) error {

	// save options, consumer
	fu.options = options
	fu.kubeConsumer = kubeConsumer

	fu.logger.InfoWith("Updating function", "name", options.Common.Identifier)

	resourceName, _, err := nuctl.ParseResourceIdentifier(options.Common.Identifier)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	// get specific function CR
	functioncrInstance, err := fu.kubeConsumer.FunctioncrClient.Get(options.Common.Namespace, resourceName)
	if err != nil {
		return errors.Wrap(err, "Failed to get function")
	}

	// if we're updating the "latest" function
	if functioncrInstance.Spec.Alias == "latest" {

		// if we need to publish - make sure alias is unset
		if options.Run.Publish {
			options.Alias = ""
		} else {

			// if the function's current alias is "latest" and alias wasn't set, set it to latest
			if options.Alias == "" {
				options.Alias = "latest"
			}
		}
	}

	// update it with the run options
	err = runner.UpdateFunctioncrWithOptions(&options.Run, functioncrInstance)
	if err != nil {
		return errors.Wrap(err, "Failed to update function")
	}

	// trigger an update
	createdFunctioncr, err := fu.kubeConsumer.FunctioncrClient.Update(functioncrInstance)

	// wait until function is processed
	// TODO: this is not proper. We need to wait until the resource version changes or something as well since
	// the function might already be processed and we will unblock immediately
	err = fu.kubeConsumer.FunctioncrClient.WaitUntilCondition(createdFunctioncr.Namespace,
		createdFunctioncr.Name,
		functioncr.WaitConditionProcessed,
		10*time.Second,
	)

	if err != nil {
		return errors.Wrap(err, "Failed to wait until function is processed")
	}

	fu.logger.InfoWith("Function updated")

	return nil
}
