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
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"

	"github.com/nuclio/nuclio-sdk"
)

type updater struct {
	logger   nuclio.Logger
	platform platform.Platform
}

func newUpdater(parentLogger nuclio.Logger, platform platform.Platform) (*updater, error) {
	newupdater := &updater{
		logger:   parentLogger.GetChild("updater"),
		platform: platform,
	}

	return newupdater, nil
}

func (u *updater) update(consumer *consumer, updateOptions *platform.UpdateOptions) error {
	u.logger.InfoWith("Updating function", "name", updateOptions.FunctionConfig.Meta.Name)

	resourceName, _, err := nuctl.ParseResourceIdentifier(updateOptions.FunctionConfig.Meta.Name)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	// get specific function CR
	functioncrInstance, err := consumer.functioncrClient.Get(updateOptions.FunctionConfig.Meta.Namespace, resourceName)
	if err != nil {
		return errors.Wrap(err, "Failed to get function")
	}

	// if we're updating the "latest" function
	if functioncrInstance.Spec.Alias == "latest" {

		// if we need to publish - make sure alias is unset
		if updateOptions.FunctionConfig.Spec.Publish {
			updateOptions.FunctionConfig.Spec.Alias = ""
		} else {

			// if the function's current alias is "latest" and alias wasn't set, set it to latest
			if updateOptions.FunctionConfig.Spec.Alias == "" {
				updateOptions.FunctionConfig.Spec.Alias = "latest"
			}
		}
	}

	// update it with the run options
	err = UpdateFunctioncrWithConfig(&updateOptions.FunctionConfig, functioncrInstance)

	if err != nil {
		return errors.Wrap(err, "Failed to update function")
	}

	// trigger an update
	createdFunctioncr, err := consumer.functioncrClient.Update(functioncrInstance)
	if err != nil {
		return errors.Wrap(err, "Failed to update function CR")
	}

	// wait until function is processed
	// TODO: this is not proper. We need to wait until the resource version changes or something as well since
	// the function might already be processed and we will unblock immediately
	err = consumer.functioncrClient.WaitUntilCondition(createdFunctioncr.Namespace,
		createdFunctioncr.Name,
		functioncr.WaitConditionProcessed,
		10*time.Second,
	)

	if err != nil {
		return errors.Wrap(err, "Failed to wait until function is processed")
	}

	u.logger.InfoWith("Function updated")

	return nil
}
