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

type Runtime interface {
	ProcessEvent(event nuclio.Event, functionLogger nuclio.Logger) (interface{}, error)

	GetFunctionLogger() nuclio.Logger

	GetStatistics() *Statistics
}

type AbstractRuntime struct {
	Logger         nuclio.Logger
	FunctionLogger nuclio.Logger
	Context        *nuclio.Context
	Statistics     Statistics
	databindings   map[string]databinding.DataBinding
}

func NewAbstractRuntime(logger nuclio.Logger, configuration *Configuration) (*AbstractRuntime, error) {
	var err error

	newAbstractRuntime := AbstractRuntime{
		Logger:         logger,
		FunctionLogger: configuration.FunctionLogger,
	}

	// create data bindings and start them (connecting to the actual data sources)
	newAbstractRuntime.databindings, err = newAbstractRuntime.createAndStartDataBindings(logger, configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create data bindings")
	}

	newAbstractRuntime.Context, err = newAbstractRuntime.createContext(newAbstractRuntime.FunctionLogger,
		configuration,
		newAbstractRuntime.databindings)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create context")
	}

	return &newAbstractRuntime, nil
}

func (ar *AbstractRuntime) GetFunctionLogger() nuclio.Logger {
	return ar.FunctionLogger
}

func (ar *AbstractRuntime) GetStatistics() *Statistics {
	return &ar.Statistics
}

func (ar *AbstractRuntime) createAndStartDataBindings(parentLogger nuclio.Logger,
	configuration *Configuration) (map[string]databinding.DataBinding, error) {

	databindings := map[string]databinding.DataBinding{}

	// create data bindings through the data binding registry
	// TODO: this should be in parallel
	for dataBindingName, dataBindingConfiguration := range configuration.Spec.DataBindings {
		databindingInstance, err := databinding.RegistrySingleton.NewDataBinding(parentLogger,
			dataBindingConfiguration.Class,
			dataBindingConfiguration.Name,
			&dataBindingConfiguration)

		if err != nil {
			return nil, errors.Wrap(err, "Failed to create data binding")
		}

		if err := databindingInstance.Start(); err != nil {
			return nil, errors.Wrap(err, "Failed to start data binding")
		}

		databindings[dataBindingName] = databindingInstance
	}

	return databindings, nil
}

func (ar *AbstractRuntime) createContext(parentLogger nuclio.Logger,
	configuration *Configuration,
	databindings map[string]databinding.DataBinding) (*nuclio.Context, error) {

	newContext := &nuclio.Context{
		Logger:      parentLogger,
		DataBinding: map[string]nuclio.DataBinding{},
	}

	// iterate through data bindings and get the context object - the thing users will actuall
	// work with in the handlers
	for databindingName, databindingInstance := range databindings {
		var err error

		newContext.DataBinding[databindingName], err = databindingInstance.GetContextObject()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get databinding context object")
		}
	}

	return newContext, nil
}
