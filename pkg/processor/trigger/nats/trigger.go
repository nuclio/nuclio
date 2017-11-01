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
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	natsio "github.com/nats-io/go-nats"
	"github.com/nuclio/nuclio-sdk"
)

type nats struct {
	trigger.AbstractTrigger
	event            Event
	configuration    *Configuration
	stop             chan bool
	natsSubscription *natsio.Subscription
}

func newTrigger(parentLogger nuclio.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {

	newTrigger := &nats{
		AbstractTrigger: trigger.AbstractTrigger{
			ID:              configuration.ID,
			Logger:          parentLogger.GetChild(configuration.ID),
			WorkerAllocator: workerAllocator,
			Class:           "async",
			Kind:            "nats",
		},
		configuration: configuration,
		stop:          make(chan bool),
	}

	return newTrigger, nil
}

func (n *nats) Start(checkpoint trigger.Checkpoint) error {
	n.Logger.InfoWith("Starting",
		"serverURL", n.configuration.serverURL,
		"topic", n.configuration.topic)

	natsConnection, err := natsio.Connect(n.configuration.serverURL)
	if err != nil {
		return errors.Wrapf(err, "Can't connect to NATS server %s", n.configuration.serverURL)
	}

	messageChan := make(chan *natsio.Msg, 64)
	n.natsSubscription, err = natsConnection.ChanSubscribe(n.configuration.topic, messageChan)
	if err != nil {
		return errors.Wrapf(err, "Can't subscribe to topic %q", n.configuration.topic)
	}
	go n.listenForMessages(messageChan)
	return nil
}

func (n *nats) Stop(force bool) (trigger.Checkpoint, error) {
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
