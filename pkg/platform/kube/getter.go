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
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"

	"github.com/nuclio/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type getter struct {
	logger   logger.Logger
	platform platform.Platform
}

func newGetter(parentLogger logger.Logger, platform platform.Platform) (*getter, error) {
	newgetter := &getter{
		logger:   parentLogger.GetChild("getter"),
		platform: platform,
	}

	return newgetter, nil
}

func (g *getter) get(consumer *consumer, getFunctionsOptions *platform.GetFunctionsOptions) ([]platform.Function, error) {
	var platformFunctions []platform.Function
	var functions []nuclioio.Function

	// if identifier specified, we need to get a single function
	if getFunctionsOptions.Name != "" {

		// get specific function CR
		function, err := consumer.nuclioClientSet.NuclioV1beta1().Functions(getFunctionsOptions.Namespace).Get(getFunctionsOptions.Name, meta_v1.GetOptions{})
		if err != nil {

			// if we didn't find the function, return an empty slice
			if apierrors.IsNotFound(err) {
				return platformFunctions, nil
			}

			return nil, errors.Wrap(err, "Failed to get function")
		}

		functions = append(functions, *function)

	} else {

		functionInstanceList, err := consumer.nuclioClientSet.NuclioV1beta1().Functions(getFunctionsOptions.Namespace).List(meta_v1.ListOptions{LabelSelector: getFunctionsOptions.Labels})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to list functions")
		}

		// convert []Function to []*Function
		functions = functionInstanceList.Items
	}

	// convert []nuclioio.Function -> function
	for functionInstanceIndex := 0; functionInstanceIndex < len(functions); functionInstanceIndex++ {
		functionInstance := functions[functionInstanceIndex]

		newFunction, err := newFunction(g.logger,
			g.platform,
			&functionInstance,
			consumer)

		if err != nil {
			return nil, err
		}

		platformFunctions = append(platformFunctions, newFunction)
	}

	// render it
	return platformFunctions, nil
}
