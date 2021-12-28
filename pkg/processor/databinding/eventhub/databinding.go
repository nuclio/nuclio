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
	"github.com/nuclio/nuclio/pkg/processor/databinding"
	"github.com/nuclio/nuclio/pkg/processor/util/eventhub"

	"github.com/Azure/go-amqp"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type eventhub struct {
	databinding.AbstractDataBinding
	configuration  *Configuration
	eventhubSender *amqp.Sender
}

func newDataBinding(parentLogger logger.Logger, configuration *Configuration) (databinding.DataBinding, error) {
	newEventhub := eventhub{
		AbstractDataBinding: databinding.AbstractDataBinding{
			Logger: parentLogger.GetChild("eventhub"),
		},
		configuration: configuration,
	}

	newEventhub.Logger.InfoWith("Creating", "configuration", configuration)

	return &newEventhub, nil
}

func (eh *eventhub) Start() error {
	session, err := eventhubutil.CreateSession(eh.configuration.Namespace,
		eh.configuration.SharedAccessKeyName,
		eh.configuration.SharedAccessKeyValue)

	if err != nil {
		return errors.Wrap(err, "Failed to create session")
	}

	// Create a sender
	eh.eventhubSender, err = session.NewSender(amqp.LinkTargetAddress(eh.configuration.EventHubName))
	if err != nil {
		return errors.Wrap(err, "Failed to create sender")
	}

	return nil
}

// GetContextObject will return the object that is injected into the context
func (eh *eventhub) GetContextObject() (interface{}, error) {
	return eh.eventhubSender, nil
}
