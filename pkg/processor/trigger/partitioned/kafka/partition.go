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

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/trigger/partitioned"

	"github.com/Shopify/sarama"
	"github.com/nuclio/logger"
)

type partition struct {
	*partitioned.AbstractPartition
	partitionID       int
	partitionConsumer sarama.PartitionConsumer
	event             Event
}

func newPartition(parentLogger logger.Logger, kafkaTrigger *kafka, partitionID int) (*partition, error) {
	var err error
	partitionName := fmt.Sprintf("partition-%d", partitionID)

	// create a partition
	newPartition := &partition{
		partitionID: partitionID,
	}

	newPartition.AbstractPartition, err = partitioned.NewAbstractPartition(parentLogger.GetChild(partitionName),
		kafkaTrigger.AbstractStream)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract partition")
	}

	newPartition.partitionConsumer, err = kafkaTrigger.consumer.ConsumePartition(kafkaTrigger.configuration.Topic,
		int32(partitionID),
		sarama.OffsetNewest)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create partition consumer")
	}

	return newPartition, nil
}

func (p *partition) Read() error {
	for kafkaMessage := range p.partitionConsumer.Messages() {

		// bind to delivery
		p.event.kafkaMessage = kafkaMessage

		// submit to worker
		p.Stream.SubmitEventToWorker(nil, p.Worker, &p.event) // nolint: errcheck
	}

	return nil
}
