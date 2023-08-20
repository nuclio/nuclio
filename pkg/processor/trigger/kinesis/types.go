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

package kinesis

import (
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
)

type Configuration struct {
	trigger.Configuration
	AccessKeyID           string
	SecretAccessKey       string
	RegionName            string
	StreamName            string
	Shards                []string
	IteratorType          string
	PollingPeriod         string
	pollingPeriodDuration time.Duration
}

func NewConfiguration(id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	var err error
	newConfiguration := Configuration{}

	// create base
	baseConfiguration, err := trigger.NewConfiguration(id, triggerConfiguration, runtimeConfiguration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create trigger configuration")
	}
	newConfiguration.Configuration = *baseConfiguration

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	if newConfiguration.IteratorType == "" {
		newConfiguration.IteratorType = "LATEST"
	}

	if err := newConfiguration.validateIteratorType(newConfiguration.IteratorType); err != nil {
		return nil, errors.Wrapf(err, "Invalid iterator type %s", newConfiguration.IteratorType)
	}

	if newConfiguration.PollingPeriod == "" {
		newConfiguration.PollingPeriod = "500ms"
	}

	newConfiguration.pollingPeriodDuration, err = time.ParseDuration(newConfiguration.PollingPeriod)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse polling period duration")
	}

	return &newConfiguration, nil
}

func (c *Configuration) validateIteratorType(iteratorType string) error {
	switch iteratorType {
	case "TRIM_HORIZON", "LATEST":
		return nil
	default:
		return errors.New("Unsupported iterator type")
	}
}
