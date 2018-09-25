/*
Copyright 2018 The Nuclio Authors.

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
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/bsm/sarama-cluster"
	"github.com/nuclio/logger"
)

type kafka struct {
	trigger.AbstractTrigger
	configuration  *Configuration
	kafkaConfig    *cluster.Config
	consumer       *cluster.Consumer
	shutdownSignal chan struct{}
}

func (k *kafka) Start(checkpoint functionconfig.Checkpoint) error {

	var err error

	k.consumer, err = k.newConsumer()
	if err != nil {
		return errors.Wrap(err, "Failed to create consumer")
	}

	k.shutdownSignal = make(chan struct{}, 1)

	// consume partitions
	go func() {
		for {
			select {
			case partition, ok := <-k.consumer.Partitions():
				if !ok {
					k.Logger.Warn("Kafka trigger shutting down due to underlying consumer shutdown")
					return
				}

				workerInstance, err := k.WorkerAllocator.Allocate(0)
				if err != nil {
					k.Logger.ErrorWith("Failed to allocate worker", "error", err)
					return
				}
				go k.consumeFromPartition(partition, workerInstance)
			case <-k.shutdownSignal:
				k.Logger.Info("Shutting down kafka trigger")
				return
			}
		}
	}()

	return err
}

func (k *kafka) Stop(force bool) (functionconfig.Checkpoint, error) {
	k.shutdownSignal <- struct{}{}
	close(k.shutdownSignal)
	err := k.consumer.Close()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to close consumer")
	}
	return nil, nil
}

func (k *kafka) GetConfig() map[string]interface{} {
	return common.StructureToMap(k.configuration)
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {
	var err error

	newTrigger := &kafka{
		configuration: configuration,
	}

	newTrigger.AbstractTrigger = trigger.AbstractTrigger{
		ID:              configuration.ID,
		Logger:          parentLogger.GetChild(configuration.ID),
		WorkerAllocator: workerAllocator,
		Class:           "async",
		Kind:            "kafka-cluster",
	}

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract stream")
	}

	newTrigger.Logger.DebugWith("Creating consumer", "url", configuration.URL)

	newTrigger.kafkaConfig, err = newTrigger.newKafkaConfig(configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create configuration")
	}

	return newTrigger, nil
}

func (k *kafka) newKafkaConfig(configuration *Configuration) (*cluster.Config, error) {

	config := cluster.NewConfig()
	config.Group.Mode = cluster.ConsumerModePartitions

	config.Consumer.Offsets.Initial = k.configuration.SaramaInitialOffset

	return config, nil
}

func (k *kafka) newConsumer() (*cluster.Consumer, error) {
	// init consumer
	brokers := []string{k.configuration.URL}
	consumer, err := cluster.NewConsumer(brokers, k.configuration.ConsumerGroup, k.configuration.Topics, k.kafkaConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create consumer")
	}

	k.Logger.DebugWith("Consumer created", "url", k.configuration.URL)
	return consumer, nil
}

func (k *kafka) consumeFromPartition(partitionConsumer cluster.PartitionConsumer, worker *worker.Worker) {
	defer k.WorkerAllocator.Release(worker)
	event := Event{}
	for message := range partitionConsumer.Messages() {
		event.kafkaMessage = message
		k.SubmitEventToWorker(nil, worker, &event) // nolint: errcheck
		k.consumer.MarkOffset(message, "")         // mark message as processed
	}
}
