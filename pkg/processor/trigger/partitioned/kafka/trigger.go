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
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/trigger/partitioned"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/Shopify/sarama"
	"github.com/nuclio/logger"
)

type kafka struct {
	*partitioned.AbstractStream
	configuration *Configuration
	consumer      sarama.Consumer
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {
	var err error

	newTrigger := &kafka{
		configuration: configuration,
	}

	newTrigger.AbstractStream, err = partitioned.NewAbstractStream(parentLogger,
		workerAllocator,
		&configuration.Configuration,
		newTrigger,
		"kafka")

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract stream")
	}

	newTrigger.Logger.DebugWith("Creating consumer", "url", configuration.URL)

	newTrigger.consumer, err = sarama.NewConsumer([]string{configuration.URL}, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create consumer")
	}

	newTrigger.Logger.DebugWith("Consumer created", "url", configuration.URL)

	return newTrigger, nil
}

func (k *kafka) CreatePartitions() ([]partitioned.Partition, error) {
	var partitions []partitioned.Partition

	// iterate over partitions and create
	for _, partitionID := range k.configuration.Partitions {

		// create the partition
		partition, err := newPartition(k.Logger, k, partitionID)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create partition")
		}

		// add partition
		partitions = append(partitions, partition)
	}

	return partitions, nil
}
