/*
Copyright 2023 The Nuclio Authors.

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
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger/partitioned"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
)

type Configuration struct {
	partitioned.Configuration
	SharedAccessKeyName  string
	SharedAccessKeyValue string
	Namespace            string
	EventHubName         string
	ConsumerGroup        string
	Partitions           []int
}

func NewConfiguration(id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	baseConfiguration, err := partitioned.NewConfiguration(id, triggerConfiguration, runtimeConfiguration)
	if err != nil {
		return nil, err
	}
	newConfiguration.Configuration = *baseConfiguration

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	if newConfiguration.ConsumerGroup == "" {
		newConfiguration.ConsumerGroup = "$Default"
	}

	return &newConfiguration, nil
}
