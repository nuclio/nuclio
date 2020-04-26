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
	"github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (u *updater) createOrUpdate(updateFunctionOptions *platform.UpdateFunctionOptions) error {
	u.logger.InfoWith("Creating/updating function with update options",
		"name", updateFunctionOptions.FunctionMeta.Name)

	// get clientset
	nuclioClientSet, err := u.consumer.getNuclioClientSet(updateFunctionOptions.AuthConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to get nuclio clientset")
	}

	functionExists := true

	// get specific function CR
	function, err := nuclioClientSet.NuclioV1beta1().
		NuclioFunctions(updateFunctionOptions.FunctionMeta.Namespace).
		Get(updateFunctionOptions.FunctionMeta.Name, meta_v1.GetOptions{})

	if err != nil {
		return errors.Wrap(err, "Failed to get function")
	}

	// create if no
	if function == nil {
		functionExists = false
	}

	// update it with spec if passed
	if updateFunctionOptions.FunctionSpec != nil {
		function.Spec = *updateFunctionOptions.FunctionSpec

		// update the spec with a new image hash to trigger pod restart. in the future this can be removed,
		// assuming the processor can reload configuration
		function.Spec.ImageHash = strconv.Itoa(int(time.Now().UnixNano()))
	}

	// update it with status - must have or waitForFunctionReadiness will continue forever
	if updateFunctionOptions.FunctionStatus == nil {
		return errors.Wrapf(err, "Update function called but no status was filled: %v", updateFunctionOptions)
	}

	function.Status = *updateFunctionOptions.FunctionStatus

	var updatedFunction *v1beta1.NuclioFunction
	if functionExists {
		// trigger an update
		updatedFunction, err = nuclioClientSet.NuclioV1beta1().
			NuclioFunctions(updateFunctionOptions.FunctionMeta.Namespace).
			Update(function)
	} else {
		// create function
		updatedFunction, err = nuclioClientSet.NuclioV1beta1().
			NuclioFunctions(updateFunctionOptions.FunctionMeta.Namespace).
			Create(function)
	}

	if err != nil {
		return errors.Wrap(err, "Failed to update function CR")
	}

	// wait for the function to be ready
	_, err = waitForFunctionReadiness(u.logger,
		u.consumer,
		updatedFunction.Namespace,
		updatedFunction.Name)

	if err != nil {
		return errors.Wrap(err, "Failed to wait for function readiness")
	}

	u.logger.InfoWith("Function updated")

	return nil
}
