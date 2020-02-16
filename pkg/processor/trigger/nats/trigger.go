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

package nats

import (
	"bytes"
	"net/url"
	"text/template"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	natsio "github.com/nats-io/go-nats"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type nats struct {
	trigger.AbstractTrigger
	event            Event
	configuration    *Configuration
	stop             chan bool
	natsSubscription *natsio.Subscription
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {
	abstractTrigger, err := trigger.NewAbstractTrigger(parentLogger.GetChild(configuration.ID),
		workerAllocator,
		&configuration.Configuration,
		"async",
		"nats",
		configuration.Name)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger := &nats{
		AbstractTrigger: abstractTrigger,
		configuration:   configuration,
		stop:            make(chan bool),
	}

	err = newTrigger.validateConfiguration()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to validate NATS trigger configuration")
	}

	return newTrigger, nil
}

func (n *nats) validateConfiguration() error {
	natsURL, err := url.Parse(n.configuration.URL)
	if err != nil {
		return errors.Wrap(err, "Failed to parse NATS URL")
	}

	if natsURL.Scheme != "nats" {
		return errors.New("Invalid URL. Must begin with 'nats://'")
	}

	return nil
}

func (n *nats) Start(checkpoint functionconfig.Checkpoint) error {
	queueName := n.configuration.QueueName
	if queueName == "" {
		queueName = "{{.Namespace}}.{{.Name}}-{{.Id}}"
	}

	queueNameTemplate, err := template.New("queueName").Parse(queueName)
	if err != nil {
		return errors.Wrap(err, "Failed to create queueName template")
	}

	var queueNameTemplateBuffer bytes.Buffer
	err = queueNameTemplate.Execute(&queueNameTemplateBuffer, &map[string]interface{}{
		"Namespace":   n.configuration.RuntimeConfiguration.Meta.Namespace,
		"Name":        n.configuration.RuntimeConfiguration.Meta.Name,
		"Id":          n.configuration.ID,
		"Labels":      n.configuration.RuntimeConfiguration.Meta.Labels,
		"Annotations": n.configuration.RuntimeConfiguration.Meta.Annotations,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to execute queueName template")
	}

	queueName = queueNameTemplateBuffer.String()
	n.Logger.InfoWith("Starting",
		"serverURL", n.configuration.URL,
		"topic", n.configuration.Topic,
		"queueName", queueName)

	natsConnection, err := natsio.Connect(n.configuration.URL)
	if err != nil {
		return errors.Wrapf(err, "Can't connect to NATS server %s", n.configuration.URL)
	}

	messageChan := make(chan *natsio.Msg, 64)
	n.natsSubscription, err = natsConnection.ChanQueueSubscribe(n.configuration.Topic, n.configuration.QueueName, messageChan)
	if err != nil {
		return errors.Wrapf(err, "Can't subscribe to topic %q in queue %q", n.configuration.Topic, queueName)
	}
	go n.listenForMessages(messageChan)
	return nil
}

func (n *nats) Stop(force bool) (functionconfig.Checkpoint, error) {
	n.stop <- true
	return nil, n.natsSubscription.Unsubscribe()
}

func (n *nats) listenForMessages(messageChan chan *natsio.Msg) {
	for {
		select {
		case natsMessage := <-messageChan:
			n.event.natsMessage = natsMessage
			// process the event, don't really do anything with response
			_, submitError, processError := n.AllocateWorkerAndSubmitEvent(&n.event, n.Logger, 10*time.Second)
			if submitError != nil {
				n.Logger.ErrorWith("Can't submit event", "error", submitError)
			}
			if processError != nil {
				n.Logger.ErrorWith("Can't process event", "error", processError)
			}
		case <-n.stop:
			return
		}
	}
}

func (n *nats) GetConfig() map[string]interface{} {
	return common.StructureToMap(n.configuration)
}
