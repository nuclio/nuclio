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

	natsio "github.com/nats-io/nats.go"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type nats struct {
	trigger.AbstractTrigger
	configuration    *Configuration
	stop             chan bool
	natsSubscription *natsio.Subscription
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration,
	restartTriggerChan chan trigger.Trigger) (trigger.Trigger, error) {
	abstractTrigger, err := trigger.NewAbstractTrigger(parentLogger.GetChild(configuration.ID),
		workerAllocator,
		&configuration.Configuration,
		"async",
		"nats",
		configuration.Name,
		restartTriggerChan)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger := &nats{
		AbstractTrigger: abstractTrigger,
		configuration:   configuration,
		stop:            make(chan bool),
	}
	newTrigger.AbstractTrigger.Trigger = newTrigger

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

			// submit the event to the worker in the background and continue,
			// as we don't mark anything when processing is done
			go func() {
				event := &Event{
					natsMessage: natsMessage,
				}

				// allocate a worker
				workerInstance, err := n.WorkerAllocator.Allocate(time.Duration(*n.configuration.WorkerAvailabilityTimeoutMilliseconds) * time.Millisecond)
				if err != nil {
					n.UpdateStatistics(false, 1)
					n.Logger.ErrorWith("Failed to allocate worker", "error", err)
					return
				}

				// submit the event to the worker, don't really do anything with response
				_, processErr := n.SubmitEventToWorker(nil, workerInstance, event)
				if processErr != nil {
					n.Logger.ErrorWith("Can't process event", "error", processErr)
				}

				// release the worker
				n.WorkerAllocator.Release(workerInstance)
			}()

		case <-n.stop:
			return
		}
	}
}

func (n *nats) GetConfig() map[string]interface{} {
	return common.StructureToMap(n.configuration)
}
