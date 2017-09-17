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
	"os"
	"strings"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/spf13/viper"
)

// Copied from functioncr to prevent dependencies on functioncr
type DataBinding struct {
	Name    string            `json:"name"`
	Class   string            `json:"class"`
	Url     string            `json:"url"`
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
}

func NewConfiguration(configuration *viper.Viper) (*Configuration, error) {

	newConfiguration := &Configuration{
		Name:           configuration.GetString("name"),
		Description:    configuration.GetString("description"),
		Version:        configuration.GetString("version"),
		DataBindings:   map[string]*DataBinding{},
		FunctionLogger: configuration.Get("function_logger").(nuclio.Logger),
	}

	// get databindings by environment variables
	err := newConfiguration.getDataBindingsFromEnv(os.Environ(), newConfiguration.DataBindings)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to read data bindings from environment")
	}

	return newConfiguration, nil
}

func (c *Configuration) getDataBindingsFromEnv(envs []string, dataBindings map[string]*DataBinding) error {
	dataBindingPrefix := "NUCLIO_DATA_BINDING_"

	// iterate over env
	for _, env := range envs {
		envKeyValue := strings.Split(env, "=")
		envKey := envKeyValue[0]
		envValue := envKeyValue[1]

		// check if it starts with data binding prefix. if it doesn't do nothing
		if !strings.HasPrefix(envKey, dataBindingPrefix) {
			continue
		}

		// strip the prefix
		envKey = envKey[len(dataBindingPrefix):]

		// look for the known postfixes
		for _, postfix := range []string{
			"CLASS",
			"URL",
		} {

			// skip if it's not the postfix we're looking for
			if !strings.HasSuffix(envKey, postfix) {
				continue
			}

			// get the data binding name
			dataBindingName := envKey[:len(envKey)-len(postfix)-1]

			var dataBinding *DataBinding

			// get or create/insert the data binding
			dataBinding, ok := dataBindings[dataBindingName]
			if !ok {

				// create a new one and shove to map
				dataBinding = &DataBinding{}
				dataBindings[dataBindingName] = dataBinding
			}

			switch postfix {
			case "CLASS":
				dataBinding.Class = envValue
			case "URL":
				dataBinding.Url = envValue
			}
		}
	}

	return nil
}
