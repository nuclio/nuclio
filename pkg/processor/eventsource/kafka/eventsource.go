// +build kafka

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

package kafka

import (
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"github.com/nuclio/nuclio/pkg/util/common"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/nuclio/nuclio-sdk"
)

type kafkaEventSource struct {
	eventsource.AbstractEventSource
	event         Event
	configuration *Configuration
	worker        *worker.Worker
}

func newEventSource(parentLogger nuclio.Logger,
	workerAllocator worker.WorkerAllocator,
	configuration *Configuration) (eventsource.EventSource, error) {

	newEventSource := kafkaEventSource{
		AbstractEventSource: eventsource.AbstractEventSource{
			ID:              configuration.ID,
			Logger:          parentLogger.GetChild(configuration.ID).(nuclio.Logger),
			WorkerAllocator: workerAllocator,
			Class:           "async",
			Kind:            "kafka",
		},
		configuration: configuration,
	}

	return &newEventSource, nil
}

func (k *kafkaEventSource) Start(checkpoint eventsource.Checkpoint) error {
	var err error

	k.Logger.InfoWith("Starting", "broker", k.configuration.Broker)

	// get a worker, we'll be using this one always
	k.worker, err = k.WorkerAllocator.Allocate(10 * time.Second)
	if err != nil {
		return errors.Wrap(err, "Failed to allocate worker")
	}

	var consumer *kafka.Consumer
	if consumer, err = k.createConsumer(); err != nil {
		return errors.Wrap(err, "Failed to create consumer")
	}

	// start listening for published messages
	go k.handleMessages(consumer)

	return nil
}

func (k *kafkaEventSource) Stop(force bool) (eventsource.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (k *kafkaEventSource) GetConfig() map[string]interface{} {
	return common.StructureToMap(k.configuration)
}

func (k *kafkaEventSource) createConsumer() (*kafka.Consumer, error) {
	var err error

	k.Logger.InfoWith("Creating broker resources",
		"broker", k.configuration.Broker,
		"group", k.configuration.Group,
		"topics", k.configuration.Topics)

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":               k.configuration.Broker,
		"group.id":                        k.configuration.Group,
		"session.timeout.ms":              6000,
		"go.events.channel.enable":        true,
		"go.application.rebalance.enable": true,
		"default.topic.config":            kafka.ConfigMap{"auto.offset.reset": "earliest"}})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create connection to server")
	}

	k.Logger.DebugWith("Connected to server", "broker", k.configuration.Broker)

	if err := consumer.SubscribeTopics(k.configuration.Topics, nil); err != nil {
		return nil, errors.Wrap(err, "Failed to subscribe to topics")
	}

	return consumer, nil
}

func (k *kafkaEventSource) handleMessages(consumer *kafka.Consumer) {
	for {
		select {
		case kafkaEvent := <-consumer.Events():

			// bind to delivery
			k.event.kafkaEvent = kafkaEvent

			// submit to worker
			_, submitError, _ := k.SubmitEventToWorker(&k.event, nil, 10*time.Second)

			if submitError != nil {
				k.Logger.ErrorWith("Failed to submit to worker", "error", submitError)
			}
		}
	}
}
