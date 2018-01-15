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

package runtime

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/databinding"

	"github.com/nuclio/nuclio-sdk"
)

func newContext(parentLogger nuclio.Logger, configuration *Configuration) (*nuclio.Context, error) {

	newContext := &nuclio.Context{
		Logger:      parentLogger,
		DataBinding: map[string]nuclio.DataBinding{},
	}

	// create data bindings through the data binding registry
	for dataBindingName, dataBindingConfiguration := range configuration.Spec.DataBindings {
		databindingInstance, err := databinding.RegistrySingleton.NewDataBinding(parentLogger,
			dataBindingConfiguration.Class,
			dataBindingConfiguration.Name,
			&dataBindingConfiguration)

		if err != nil {
			return nil, errors.Wrap(err, "Failed to create data binding")
		}

		newContext.DataBinding[dataBindingName] = databindingInstance
	}

	return newContext, nil
}
