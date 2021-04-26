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

package pubsub

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
)

const DefaultSubscriptionIDPrefix string = "nuclio-pub"

type Subscription struct {
	Topic       string
	IDPrefix    string
	Shared      bool
	AckDeadline string
	SkipCreate  bool

	// https://godoc.org/cloud.google.com/go/pubsub#ReceiveSettings
	MaxNumWorkers int
	Synchronous   bool
}

type Configuration struct {
	trigger.Configuration
	Subscriptions []Subscription
	ProjectID     string
	AckDeadline   string
	Credentials   trigger.Secret
	NoCredentials bool
}

func NewConfiguration(id string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *trigger.NewConfiguration(id, triggerConfiguration, runtimeConfiguration)

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	for subscriptionIdx, subscription := range newConfiguration.Subscriptions {

		if subscription.Topic == "" {
			return nil, errors.New("Subscription topic must be set")
		}

		if subscription.IDPrefix == "" {
			subscription.IDPrefix = DefaultSubscriptionIDPrefix
		}

		if subscription.MaxNumWorkers == 0 {
			newConfiguration.Subscriptions[subscriptionIdx].MaxNumWorkers = 1
		}
	}

	return &newConfiguration, nil
}
