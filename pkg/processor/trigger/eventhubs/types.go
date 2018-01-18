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

package eventhubs

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/mitchellh/mapstructure"
)

type Configuration struct {
	trigger.Configuration
	SharedAccessKeyName  string
	SharedAccessKeyValue string
	Namespace            string
	EventHubName         string
	ConsumerGroup        string
	Partitions           []int
}

func NewConfiguration(ID string, triggerConfiguration *functionconfig.Trigger) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *trigger.NewConfiguration(ID, triggerConfiguration)

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	// TODO: validate

	return &newConfiguration, nil
}
