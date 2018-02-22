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
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"

	"github.com/nuclio/logger"
)

type updater struct {
	logger   logger.Logger
	platform platform.Platform
	consumer *consumer
}

func newUpdater(parentLogger logger.Logger, consumer *consumer, platform platform.Platform) (*updater, error) {
	newupdater := &updater{
		logger:   parentLogger.GetChild("updater"),
		platform: platform,
		consumer: consumer,
	}

	return newupdater, nil
}

func (u *updater) update(updateOptions *platform.UpdateOptions) error {
	u.logger.InfoWith("Updating function", "name", updateOptions.FunctionMeta.Name)

	// get specific function CR
	functioncrInstance, err := u.consumer.functioncrClient.Get(updateOptions.FunctionMeta.Namespace,
		updateOptions.FunctionMeta.Name)

	if err != nil {
		return errors.Wrap(err, "Failed to get function")
	}

	// update it with spec if passed
	if updateOptions.FunctionSpec != nil {
		functioncrInstance.Spec = *updateOptions.FunctionSpec
	}

	// update it with status if passed
	if updateOptions.FunctionStatus != nil {
		functioncrInstance.Status.Status = *updateOptions.FunctionStatus
	}

	// trigger an update
	createdFunctioncr, err := u.consumer.functioncrClient.Update(functioncrInstance)
	if err != nil {
		return errors.Wrap(err, "Failed to update function CR")
	}

	// wait until function is ready
	timeout := 60 * time.Second
	err = u.consumer.functioncrClient.WaitUntilCondition(createdFunctioncr.Namespace,
		createdFunctioncr.Name,
		functioncr.WaitConditionReady,
		&timeout,
	)

	if err != nil {
		return errors.Wrap(err, "Failed to wait until function is ready")
	}

	u.logger.InfoWith("Function updated")

	return nil
}
