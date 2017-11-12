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
	"github.com/nuclio/nuclio-sdk"
	"github.com/spf13/viper"
)

type Statistics struct {
	DurationMilliSecondsSum   uint64
	DurationMilliSecondsCount uint64
}

func (s *Statistics) DiffFrom(prev *Statistics) Statistics {
	return Statistics{
		DurationMilliSecondsSum:   s.DurationMilliSecondsSum - prev.DurationMilliSecondsSum,
		DurationMilliSecondsCount: s.DurationMilliSecondsCount - prev.DurationMilliSecondsCount,
	}
}

// Copied from functioncr to prevent dependencies on functioncr
type DataBinding struct {
	Name    string            `json:"name"`
	Class   string            `json:"class"`
	URL     string            `json:"url"`
	Path    string            `json:"path,omitempty"`
	Query   string            `json:"query,omitempty"`
	Secret  string            `json:"secret,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

type Configuration struct {
	Name           string
	Version        string
	Description    string
	DataBindings   map[string]*DataBinding
	FunctionLogger nuclio.Logger
	Handler        string
}

func NewConfiguration(configuration *viper.Viper) (*Configuration, error) {

	newConfiguration := &Configuration{
		Name:           configuration.GetString("name"),
		Description:    configuration.GetString("description"),
		Version:        configuration.GetString("version"),
		DataBindings:   map[string]*DataBinding{},
		FunctionLogger: configuration.Get("function_logger").(nuclio.Logger),
		Handler:        configuration.GetString("handler"),
	}

	// get databindings, as injected by processor
	dataBindingsConfigurationsViper := configuration.Get("dataBindings").(*viper.Viper)
	dataBindingsConfigurations := dataBindingsConfigurationsViper.GetStringMap("")

	for dataBindingID := range dataBindingsConfigurations {
		var dataBinding DataBinding
		dataBindingsConfiguration := dataBindingsConfigurationsViper.Sub(dataBindingID)

		// set the ID of the trigger
		dataBinding.Name = dataBindingID
		dataBinding.Class = dataBindingsConfiguration.GetString("class")
		dataBinding.URL = dataBindingsConfiguration.GetString("url")
		dataBinding.Secret = dataBindingsConfiguration.GetString("secret")

		newConfiguration.DataBindings[dataBindingID] = &dataBinding
	}

	return newConfiguration, nil
}
