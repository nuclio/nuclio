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
	"github.com/nuclio/nuclio/pkg/platform/kube/functioncr"

	"github.com/nuclio/nuclio-sdk"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type getter struct {
	logger   nuclio.Logger
	platform platform.Platform
}

func newGetter(parentLogger nuclio.Logger, platform platform.Platform) (*getter, error) {
	newgetter := &getter{
		logger:   parentLogger.GetChild("getter").(nuclio.Logger),
		platform: platform,
	}

	return newgetter, nil
}

func (g *getter) get(consumer *consumer, getOptions *platform.GetOptions) ([]platform.Function, error) {
	functions := []platform.Function{}
	functioncrInstances := []functioncr.Function{}

	// if identifier specified, we need to get a single function
	if getOptions.Identifier != "" {

		// get specific function CR
		function, err := consumer.functioncrClient.Get(getOptions.Namespace, getOptions.Identifier)
		if err != nil {

			// if we didn't find the function, return an empty slice
			if apierrors.IsNotFound(err) {
				return functions, nil
			}

			return nil, errors.Wrap(err, "Failed to get function")
		}

		functioncrInstances = append(functioncrInstances, *function)

	} else {

		functioncrInstanceList, err := consumer.functioncrClient.List(getOptions.Namespace,
			&meta_v1.ListOptions{LabelSelector: getOptions.Labels})

		if err != nil {
			return nil, errors.Wrap(err, "Failed to list functions")
		}

		// convert []Function to []*Function
		functioncrInstances = functioncrInstanceList.Items
	}

	// convert []functioncr.Function -> function
	for _, functioncrInstance := range functioncrInstances {
		functions = append(functions, &function{
			Function: functioncrInstance,
			consumer: consumer,
		})
	}

	// render it
	return functions, nil
}
