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
	"github.com/nuclio/nuclio/pkg/util/common"

	"github.com/spf13/viper"
)

// TODO: this should be a whole thing. for now this should do
type DataBinding struct {
	Class string
	URL   string
}

type Configuration struct {
	Name         string
	Version      string
	Description  string
	DataBindings []DataBinding
}

func NewConfiguration(configuration *viper.Viper) (*Configuration, error) {

	newConfiguration := &Configuration{
		Name:        configuration.GetString("name"),
		Description: configuration.GetString("description"),
		Version:     configuration.GetString("version"),
	}

	// read data bindings
	dataBindings := common.GetObjectSlice(configuration, "data_bindings")
	for _, dataBinding := range dataBindings {
		newDataBinding := DataBinding{
			Class: dataBinding["class"].(string),
			URL:   dataBinding["url"].(string),
		}

		newConfiguration.DataBindings = append(newConfiguration.DataBindings, newDataBinding)
	}

	return newConfiguration, nil
}
