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

package eventhubs

import (
	"context"
	"fmt"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/nuclio-sdk"
	"pack.ag/amqp"
)

type partition struct {
	logger      nuclio.Logger
	ehTrigger   *eventhubs
	partitionID string
	worker      *worker.Worker
}

func newPartition(parentLogger nuclio.Logger, ehTrigger *eventhubs, partitionID string) (*partition, error) {
	var err error

	newPartition := &partition{
		logger:      parentLogger.GetChild(fmt.Sprintf("partition-%s", partitionID)),
		ehTrigger:   ehTrigger,
		partitionID: partitionID,
	}

	newPartition.worker, err = ehTrigger.WorkerAllocator.Allocate(0)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to allocate worker")
	}

	return newPartition, nil
}

func (p *partition) readFromPartition() error {
	p.logger.DebugWith("Starting to read from partition")

	session, err := p.ehTrigger.ehClient.NewSession()
	if err != nil {
		errors.Wrap(err, "Creating AMQP session:")

	}

	ctx := context.Background()

	receiver, err := session.NewReceiver(
		amqp.LinkAddress(fmt.Sprintf("/%s/ConsumerGroups/%s/Partitions/%s", p.ehTrigger.configuration.EventHubName, p.ehTrigger.configuration.ConsumerGroup, p.partitionID)),
		amqp.LinkCredit(10),
	)
	if err != nil {
		errors.Wrap(err, "Creating receiver link:")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {

		// Receive next message
		msg, err := receiver.Receive(ctx)
		if err != nil {
			errors.Wrap(err, "Error Reading message from AMQP:")

		}

		// Accept message
		msg.Accept()

		// TODO: event pool
		event := Event{
			body: msg.Data,
		}

		// process the event, don't really do anything with response
		p.ehTrigger.SubmitEventToWorker(nil, p.worker, &event)

	}
}
