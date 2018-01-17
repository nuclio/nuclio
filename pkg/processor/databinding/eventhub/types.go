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

package eventhub

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/databinding"

	"github.com/mitchellh/mapstructure"
)

type Configuration struct {
	databinding.Configuration
	Namespace            string `json:"namespace,omitempty"`
	SharedAccessKeyName  string `json:"sharedAccessKeyName,omitempty"`
	SharedAccessKeyValue string `json:"sharedAccessKeyValue,omitempty"`
	EventHubName         string `json:"eventHubName,omitempty"`
}

func NewConfiguration(ID string, databindingConfiguration *functionconfig.DataBinding) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *databinding.NewConfiguration(ID, databindingConfiguration)

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	return &newConfiguration, nil
}
