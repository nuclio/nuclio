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

package rabbitmq

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
	ExchangeName      string
	QueueName         string
	Topics            []string
	ReconnectDuration string
	ReconnectInterval string
	DurableExchange   bool
	DurableQueue      bool

	reconnectDuration time.Duration
	reconnectInterval time.Duration
}

func NewConfiguration(id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	var err error
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *trigger.NewConfiguration(id, triggerConfiguration, runtimeConfiguration)

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	// defaults
	if newConfiguration.ReconnectDuration == "" {
		newConfiguration.ReconnectDuration = "5m"
	}
	if newConfiguration.ReconnectInterval == "" {
		newConfiguration.ReconnectInterval = "15s"
	}

	newConfiguration.reconnectDuration, err = time.ParseDuration(newConfiguration.ReconnectDuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse reconnect duration")
	}
	newConfiguration.reconnectInterval, err = time.ParseDuration(newConfiguration.ReconnectInterval)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse reconnect interval")
	}

	// TODO: validate
	return &newConfiguration, nil
}
