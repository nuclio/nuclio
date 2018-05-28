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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
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

func (k *kafka) CreatePartitions() (map[int]partitioned.Partition, error) {
	// iterate over partitions and create
	for _, partitionConfig := range k.configuration.Partitions {
		if err := k.AddPartition(&partitionConfig); err != nil {
			return nil, err
		}

	}

	return k.Partitions, nil
}

func (k *kafka) Stop(force bool) (functionconfig.Checkpoint, error) {
	offsets := make(map[int]functionconfig.Checkpoint)

	for partitionID, partition := range k.Partitions {
		partitionConfig := &functionconfig.Partition{
			ID: fmt.Sprintf("%d", partition.GetID()),
		}
		checkpoint, err := k.RemovePartition(partitionConfig)
		if err != nil {
			return nil, err
		}
		offsets[partitionID] = checkpoint
	}

	// Must be called after we close all partitions
	if err := k.consumer.Close(); err != nil {
		return nil, errors.Wrap(err, "Can't close Kafka consumer")
	}

	out, err := json.Marshal(offsets)
	if err != nil {
		k.Logger.ErrorWith("Can't decode offsets to JSON", "err", err)
		return nil, errors.Wrap(err, "Can't encode offsets to JSON")
	}

	checkpoint := string(out)
	return &checkpoint, nil
}

// AddPartition adds a new partition
func (k *kafka) AddPartition(partitionConfig *functionconfig.Partition) error {
	partition, err := k.startPartitionFromConfig(partitionConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to create partition")
	}

	// add partition
	k.Partitions[partition.GetID()] = partition
	return nil
}

// RemovePartition removes a partition
func (k *kafka) RemovePartition(partitionConfig *functionconfig.Partition) (functionconfig.Checkpoint, error) {
	kafkaPartition, err := k.findPartition(partitionConfig)
	if err != nil {
		return nil, err
	}

	checkpoint := kafkaPartition.Stop()
	k.WorkerAllocator.Release(kafkaPartition.Worker)

	delete(k.Partitions, kafkaPartition.GetID())
	return checkpoint, nil
}

// UpdatePartition changes a partition
func (k *kafka) UpdatePartition(partitionConfig *functionconfig.Partition) error {
	if partitionConfig.Checkpoint == nil {
		return errors.Errorf("nil checkpoint on update (%v)", partitionConfig)
	}

	_, err := strconv.Atoi(*partitionConfig.Checkpoint)
	if err != nil {
		return errors.Errorf("bad checkpoint on update (%v) - %s", partitionConfig, err)
	}

	kafkaPartition, err := k.findPartition(partitionConfig)
	if err != nil {
		return err
	}

	kafkaPartition.Stop()
	k.WorkerAllocator.Release(kafkaPartition.Worker)

	partition, err := k.startPartitionFromConfig(partitionConfig)
	if err != nil {
		return err
	}

	k.Partitions[partition.GetID()] = partition
	return nil
}

func (k *kafka) findPartition(partitionConfig *functionconfig.Partition) (*partition, error) {
	partitionID, err := strconv.Atoi(partitionConfig.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "Bad partition id %s (%s)", partitionConfig.ID, err)
	}

	abstractPartition, ok := k.Partitions[partitionID]
	if !ok {
		return nil, errors.Errorf("Can't find partition %d", partitionID)
	}

	kafkaPartition, ok := abstractPartition.(*partition)
	if !ok {
		return nil, errors.Errorf("Can't convert partition %d to Kafka partition", partitionID)
	}

	return kafkaPartition, nil
}

// Create new partition and start reading from it
func (k *kafka) startPartitionFromConfig(partitionConfig *functionconfig.Partition) (*partition, error) {
	partition, err := newPartition(k.Logger, k, *partitionConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create partition")
	}

	go func() {
		if err := partition.Read(); err != nil {
			k.Logger.ErrorWith("Failed to read from partition", "err", err)
		}
	}()

	return partition, nil
}
