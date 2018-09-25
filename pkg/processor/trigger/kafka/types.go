/*
Copyright 2018 The Nuclio Authors.

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

package kafka

import (
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/Shopify/sarama"
	"github.com/mitchellh/mapstructure"
)

type Configuration struct {
	trigger.Configuration
	Topics              []string
	ConsumerGroup       string
	SaramaInitialOffset int64
}

func NewConfiguration(ID string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *trigger.NewConfiguration(ID, triggerConfiguration, runtimeConfiguration)

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	if len(newConfiguration.Topics) == 0 {
		return nil, errors.New("Topics must be set")
	}

	if newConfiguration.ConsumerGroup == "" {
		return nil, errors.New("Consumer group must be set")
	}

	var err error
	newConfiguration.SaramaInitialOffset, err = resolveInitialOffset(newConfiguration.Attributes)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve initial offset")
	}

	return &newConfiguration, nil
}

func resolveInitialOffset(attrs map[string]interface{}) (int64, error) {
	initialOffsetInterface := attrs["initialOffset"]
	var initialOffset string
	if initialOffsetInterface == nil {
		return sarama.OffsetNewest, nil
	}
	initialOffset, ok := initialOffsetInterface.(string)
	if !ok {
		return 0, errors.New("initialOffset must be a string")
	}
	if lower := strings.ToLower(initialOffset); lower == "earliest" {
		return sarama.OffsetOldest, nil
	} else if lower == "latest" {
		return sarama.OffsetNewest, nil
	} else {
		return 0, errors.Errorf("initialOffset must be either 'earliest' or 'latest', not '%s'", initialOffset)
	}
}
