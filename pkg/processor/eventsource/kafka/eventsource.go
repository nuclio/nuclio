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

	"github.com/Shopify/sarama"
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

	k.Logger.InfoWith("Starting", "host", k.configuration.Host)

	// get a worker, we'll be using this one always
	k.worker, err = k.WorkerAllocator.Allocate(10 * time.Second)
	if err != nil {
		return errors.Wrap(err, "Failed to allocate worker")
	}

	consumer, err := sarama.NewConsumer([]string{k.configuration.Host}, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to create consumer")
	}

	partitionConsumer, err := consumer.ConsumePartition(k.configuration.Topic, 0, sarama.OffsetNewest)

	if err != nil {
		return errors.Wrap(err, "Failed to create partition consumer")
	}

	// start listening for published messages
	go k.handleMessages(partitionConsumer)

	return nil
}

func (k *kafkaEventSource) Stop(force bool) (eventsource.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (k *kafkaEventSource) GetConfig() map[string]interface{} {
	return common.StructureToMap(k.configuration)
}

func (k *kafkaEventSource) handleMessages(partitionConsumer sarama.PartitionConsumer) {
	for {
		select {
		case kafkaMessage := <-partitionConsumer.Messages():

			// bind to delivery
			k.event.kafkaMessage = kafkaMessage

			// submit to worker
			_, submitError, _ := k.SubmitEventToWorker(&k.event, nil, 10*time.Second)

			if submitError != nil {
				k.Logger.ErrorWith("Failed to submit to worker", "error", submitError)
			}
		}
	}
}
