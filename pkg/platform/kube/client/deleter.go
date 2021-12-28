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
	"github.com/nuclio/nuclio/pkg/nuctl"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Deleter struct {
	logger   logger.Logger
	platform platform.Platform
}

func NewDeleter(parentLogger logger.Logger, platform platform.Platform) (*Deleter, error) {
	newDeleter := &Deleter{
		logger:   parentLogger.GetChild("deleter"),
		platform: platform,
	}

	return newDeleter, nil
}

func (d *Deleter) Delete(ctx context.Context, consumer *Consumer, deleteFunctionOptions *platform.DeleteFunctionOptions) error {
	var err error

	resourceName, _, err := nuctl.ParseResourceIdentifier(deleteFunctionOptions.FunctionConfig.Meta.Name)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	// get clientset
	nuclioClientSet, err := consumer.getNuclioClientSet(deleteFunctionOptions.AuthConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to get nuclio clientset")
	}

	var deletePreconditions *metav1.Preconditions
	if deleteFunctionOptions.FunctionConfig.Meta.ResourceVersion != "" {
		deletePreconditions = &metav1.Preconditions{
			ResourceVersion: &deleteFunctionOptions.FunctionConfig.Meta.ResourceVersion,
		}
	}

	// get specific function CR
	if err := nuclioClientSet.
		NuclioV1beta1().
		NuclioFunctions(deleteFunctionOptions.FunctionConfig.Meta.Namespace).
		Delete(ctx, resourceName, metav1.DeleteOptions{
			Preconditions: deletePreconditions,
		}); err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "Failed to delete function CR")
	}

	d.logger.InfoWithCtx(ctx, "Function deleted", "name", resourceName)

	return nil
}
