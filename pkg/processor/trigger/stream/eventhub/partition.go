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
	"context"
	"fmt"

	"github.com/nuclio/amqp"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/trigger/stream"

	"github.com/nuclio/logger"
)

type partition struct {
	*stream.AbstractPartition
	partitionID     int
	event           Event
	eventhubTrigger *eventhub
}

func newPartition(parentLogger logger.Logger, eventhubTrigger *eventhub, partitionID int) (*partition, error) {
	var err error
	partitionName := fmt.Sprintf("partition-%d", partitionID)

	// create a partition
	newPartition := &partition{
		partitionID: partitionID,
	}

	newPartition.AbstractPartition, err = stream.NewAbstractPartition(parentLogger.GetChild(partitionName),
		eventhubTrigger.AbstractStream)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract partition")
	}

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create partition consumer")
	}

	return newPartition, nil
}

func (p *partition) Read() error {
	p.Logger.DebugWith("Starting to read from partition")

	session := p.eventhubTrigger.eventhubSession

	ctx := context.Background()

	address := fmt.Sprintf("/%s/ConsumerGroups/%s/Partitions/%d",
		p.eventhubTrigger.configuration.EventHubName,
		p.eventhubTrigger.configuration.ConsumerGroup,
		p.partitionID)

	receiver, err := session.NewReceiver(
		amqp.LinkSourceAddress(address),
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
		p.eventhubTrigger.SubmitEventToWorker(nil, p.Worker, &event)
	}

	return nil
}
