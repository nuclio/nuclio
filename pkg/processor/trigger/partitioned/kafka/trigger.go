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
	"github.com/mitchellh/mapstructure"
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

	kafkaConfig, err := newTrigger.newKafkaConfig(configuration.Attributes)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create configuration")
	}

	newTrigger.consumer, err = sarama.NewConsumer([]string{configuration.URL}, kafkaConfig)
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

type kafkaAttribures struct {
	SASL struct {
		Enable   bool
		User     string
		Password string
	}
}

func (k *kafka) newKafkaConfig(attributes map[string]interface{}) (*sarama.Config, error) {
	kafkaConfig := sarama.NewConfig()
	if len(attributes) == 0 {
		return kafkaConfig, nil
	}

	userOptions := &kafkaAttribures{}
	decoderConfig := &mapstructure.DecoderConfig{
		Metadata: &mapstructure.Metadata{},
		Result:   userOptions,
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Can't create mapstructure decoder")
	}

	err = decoder.Decode(attributes)
	if err != nil {
		return nil, errors.Wrapf(err, "Can't update configuration from %+v", attributes)
	}

	if len(decoderConfig.Metadata.Unused) > 0 {
		k.Logger.WarnWith("Unused attributes in kakfa configuration", "unused", decoderConfig.Metadata.Unused)
	}

	// Copy from user configuration to sarama (not to leak implementation to config)
	kafkaConfig.Net.SASL.Enable = userOptions.SASL.Enable
	kafkaConfig.Net.SASL.User = userOptions.SASL.User
	kafkaConfig.Net.SASL.Password = userOptions.SASL.Password

	return kafkaConfig, nil
}
