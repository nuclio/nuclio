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
	"fmt"
	"strconv"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger/partitioned"

	"github.com/Shopify/sarama"
	"github.com/nuclio/logger"
)

type partition struct {
	*partitioned.AbstractPartition
	partitionID       int
	offset            int64
	partitionConsumer sarama.PartitionConsumer
	event             Event
	stopChan          chan bool
}

func newPartition(parentLogger logger.Logger, kafkaTrigger *kafka, partitionConfig functionconfig.Partition) (*partition, error) {
	var err error

	partitionID, err := strconv.Atoi(partitionConfig.ID)
	if err != nil {
		parentLogger.ErrorWith("Bad partition ID", "id", partitionConfig.ID, "err", err)
		return nil, errors.Wrapf(err, "Bad partition id (%s) - %s", partitionConfig.ID, err)
	}

	offset := sarama.OffsetNewest
	if partitionConfig.Checkpoint != nil {
		intOffset, convErr := strconv.Atoi(*partitionConfig.Checkpoint)
		if convErr != nil {
			parentLogger.ErrorWith("Bad partition checkpoint", "checkpoint", *partitionConfig.Checkpoint, "err", convErr)
			return nil, errors.Wrapf(convErr, "Bad partition checkpoint (%s) - %s", *partitionConfig.Checkpoint, err)
		}
		offset = int64(intOffset)
	}

	partitionName := fmt.Sprintf("partition-%d", partitionID)

	// create a partition
	newPartition := &partition{
		partitionID: partitionID,
		offset:      offset,
		stopChan:    make(chan bool),
	}

	newPartition.AbstractPartition, err = partitioned.NewAbstractPartition(parentLogger.GetChild(partitionName),
		kafkaTrigger.AbstractStream)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract partition")
	}

	newPartition.partitionConsumer, err = kafkaTrigger.consumer.ConsumePartition(
		kafkaTrigger.configuration.Topic,
		int32(partitionID),
		offset)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create partition consumer")
	}

	return newPartition, nil
}

func (p *partition) Read() error {
	messageChan := p.partitionConsumer.Messages()
	errorChan := p.partitionConsumer.Errors()
	for {
		select {
		case kafkaMessage := <-messageChan:
			// bind to delivery
			p.event.kafkaMessage = kafkaMessage
			p.offset = kafkaMessage.Offset

			// submit to worker
			p.Stream.SubmitEventToWorker(nil, p.Worker, &p.event) // nolint: errcheck
		case err := <-errorChan:
			return err
		case <-p.stopChan:
			return p.partitionConsumer.Close()
		}
	}
}

func (p *partition) Stop() {
	close(p.stopChan)
}
