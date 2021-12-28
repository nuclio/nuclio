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

package client

import (
	"context"
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Updater struct {
	logger   logger.Logger
	platform platform.Platform
	consumer *Consumer
}

func NewUpdater(parentLogger logger.Logger, consumer *Consumer, platform platform.Platform) (*Updater, error) {
	newUpdater := &Updater{
		logger:   parentLogger.GetChild("updater"),
		platform: platform,
		consumer: consumer,
	}

	return newUpdater, nil
}

func (u *Updater) Update(ctx context.Context, updateFunctionOptions *platform.UpdateFunctionOptions) error {
	u.logger.InfoWithCtx(ctx, "Updating function", "name", updateFunctionOptions.FunctionMeta.Name)

	// get specific function CR
	function, err := u.consumer.NuclioClientSet.NuclioV1beta1().
		NuclioFunctions(updateFunctionOptions.FunctionMeta.Namespace).
		Get(ctx, updateFunctionOptions.FunctionMeta.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to get function")
	}

	// Check OPA permissions
	permissionOptions := updateFunctionOptions.PermissionOptions
	permissionOptions.RaiseForbidden = true
	if _, err := u.platform.QueryOPAFunctionPermissions(function.Labels[common.NuclioResourceLabelKeyProjectName],
		updateFunctionOptions.FunctionMeta.Name,
		opa.ActionUpdate,
		&permissionOptions); err != nil {
		return errors.Wrap(err, "Failed authorizing OPA permissions for resource")
	}

	// update it with spec if passed
	if updateFunctionOptions.FunctionSpec != nil {
		function.Spec = *updateFunctionOptions.FunctionSpec

		// update the spec with a new image hash to trigger pod restart. in the future this can be removed,
		// assuming the processor can reload configuration
		function.Spec.ImageHash = strconv.Itoa(int(time.Now().UnixNano()))
	}

	// update it with status if passed
	if updateFunctionOptions.FunctionStatus != nil {
		function.Status = *updateFunctionOptions.FunctionStatus
	}

	// update annotations
	function.Annotations = updateFunctionOptions.FunctionMeta.Annotations

	// DO NOT update function labels because >>
	// https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#label-selector-updates

	// get clientset
	nuclioClientSet, err := u.consumer.getNuclioClientSet(updateFunctionOptions.AuthConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to get nuclio clientset")
	}

	// trigger an update
	updatedFunction, err := nuclioClientSet.
		NuclioV1beta1().
		NuclioFunctions(updateFunctionOptions.FunctionMeta.Namespace).
		Update(ctx, function, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to update function CR")
	}

	// wait for the function to be ready
	if _, err := waitForFunctionReadiness(ctx,
		u.consumer,
		updatedFunction.Namespace,
		updatedFunction.Name); err != nil {
		return errors.Wrap(err, "Failed to wait for function readiness")
	}

	u.logger.InfoWithCtx(ctx, "Function updated", "functionName", updatedFunction.Name)
	return nil
}
