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
	"fmt"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/databinding"

	"github.com/nuclio/logger"
	"github.com/omriharel/amqp"
)

type eventhub struct {
	databinding.AbstractDataBinding
	configuration *Configuration
	client        *amqp.Client
	session       *amqp.Session
	sender        *amqp.Sender
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
	var err error

	// create the client
	eh.client, err = eh.createClient()
	if err != nil {
		return errors.Wrap(err, "Failed to create client")
	}

	// open a session
	eh.session, err = eh.client.NewSession()
	if err != nil {
		return errors.Wrap(err, "Failed to create session")
	}

	// Create a sender
	eh.sender, err = eh.session.NewSender(amqp.LinkTargetAddress(eh.configuration.EventHubName))
	if err != nil {
		return errors.Wrap(err, "Failed to create sender")
	}

	return nil
}

// GetContextObject will return the object that is injected into the context
func (eh *eventhub) GetContextObject() (interface{}, error) {
	return eh.sender, nil
}

func (eh *eventhub) createClient() (*amqp.Client, error) {

	// create auth
	clientAuth := amqp.ConnSASLPlain(eh.configuration.SharedAccessKeyName,
		eh.configuration.SharedAccessKeyValue)

	// create URL
	url := fmt.Sprintf("amqps://%s.servicebus.windows.net", eh.configuration.Namespace)

	// Create client
	client, err := amqp.Dial(url, clientAuth)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to dial URL %s", eh.configuration.URL)
	}

	return client, nil
}
