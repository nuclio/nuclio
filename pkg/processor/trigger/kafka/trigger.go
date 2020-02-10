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
	"context"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/Shopify/sarama"
	"github.com/nuclio/logger"
)

type kafka struct {
	trigger.AbstractTrigger
	configuration            *Configuration
	kafkaConfig              *sarama.Config
	consumerGroup            sarama.ConsumerGroup
	shutdownSignal           chan struct{}
	partitionWorkerAllocator partitionWorkerAllocator
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {
	var err error

	loggerInstance := parentLogger.GetChild(configuration.ID)

	sarama.Logger = NewSaramaLogger(loggerInstance)

	newTrigger := &kafka{
		configuration: configuration,
	}

	newTrigger.AbstractTrigger, err = trigger.NewAbstractTrigger(loggerInstance,
		workerAllocator,
		&configuration.Configuration,
		"async",
		"kafka-cluster")
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger.Logger.DebugWith("Creating consumer",
		"brokers", configuration.brokers,
		"workerAllocationMode", configuration.WorkerAllocationMode,
		"sessionTimeout", configuration.sessionTimeout,
		"heartbeatInterval", configuration.heartbeatInterval,
		"maxProcessingTime", configuration.maxProcessingTime)

	newTrigger.kafkaConfig, err = newTrigger.newKafkaConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create configuration")
	}

	// This is the minimum required for sarama's consumer groups implementation.
	// Therefore, we do not support anything older that this version.
	newTrigger.kafkaConfig.Version = sarama.V0_10_2_0

	return newTrigger, nil
}

func (k *kafka) Start(checkpoint functionconfig.Checkpoint) error {

	var err error

	k.consumerGroup, err = k.newConsumerGroup()
	if err != nil {
		return errors.Wrap(err, "Failed to create consumer")
	}

	k.shutdownSignal = make(chan struct{}, 1)

	// start consumption in the background
	go func() {
		for {
			k.Logger.DebugWith("Starting to consume from broker", "topics", k.configuration.Topics)

			// start consuming. this will exit without error if a rebalancing occurs
			err = k.consumerGroup.Consume(context.Background(), k.configuration.Topics, k)

			if err != nil {
				k.Logger.WarnWith("Failed to consume from group, waiting before retrying", "err", err.Error())

				time.Sleep(1 * time.Second)
			} else {
				k.Logger.DebugWith("Consumer session closed (possibly due to a rebalance), re-creating")
			}
		}
	}()

	return nil
}

func (k *kafka) Stop(force bool) (functionconfig.Checkpoint, error) {
	k.shutdownSignal <- struct{}{}
	close(k.shutdownSignal)

	err := k.consumerGroup.Close()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to close consumer")
	}
	return nil, nil
}

func (k *kafka) GetConfig() map[string]interface{} {
	return common.StructureToMap(k.configuration)
}

func (k *kafka) Setup(session sarama.ConsumerGroupSession) error {
	var err error

	k.Logger.InfoWith("Starting consumer session",
		"claims", session.Claims(),
		"memberID", session.MemberID())

	k.partitionWorkerAllocator, err = k.createPartitionWorkerAllocator(session)
	if err != nil {
		return errors.Wrap(err, "Failed to create partition worker allocator")
	}

	return nil
}

func (k *kafka) Cleanup(session sarama.ConsumerGroupSession) error {
	k.Logger.InfoWith("Ending consumer session",
		"claims", session.Claims(),
		"memberID", session.MemberID())

	return nil
}

func (k *kafka) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	event := Event{}
	var submitError error

	for message := range claim.Messages() {
		event.kafkaMessage = message

		// allocate a worker for this topic/partition
		workerInstance, cookie, err := k.partitionWorkerAllocator.allocateWorker(claim.Topic(), int(claim.Partition()), nil)
		if err != nil {
			return errors.Wrap(err, "Failed to allocate worker")
		}

		// submit the event to the worker
		k.SubmitEventToWorker(nil, workerInstance, &event) // nolint: errcheck

		// mark message as processed
		session.MarkMessage(message, "")

		// release the worker from whence it came
		err = k.partitionWorkerAllocator.releaseWorker(cookie, workerInstance)
		if err != nil {
			return errors.Wrap(err, "Failed to release worker")
		}
	}

	return submitError
}

func (k *kafka) newKafkaConfig() (*sarama.Config, error) {
	config := sarama.NewConfig()

	config.Net.SASL.Enable = k.configuration.SASL.Enable
	config.Net.SASL.User = k.configuration.SASL.User
	config.Net.SASL.Password = k.configuration.SASL.Password
	config.ClientID = k.ID
	config.Consumer.Offsets.Initial = k.configuration.initialOffset
	config.Consumer.Offsets.AutoCommit.Enable = true
	config.Consumer.Group.Session.Timeout = k.configuration.sessionTimeout
	config.Consumer.Group.Heartbeat.Interval = k.configuration.heartbeatInterval
	config.Consumer.MaxProcessingTime = k.configuration.maxProcessingTime

	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "Kafka config is invalid")
	}

	return config, nil
}

func (k *kafka) newConsumerGroup() (sarama.ConsumerGroup, error) {

	consumerGroup, err := sarama.NewConsumerGroup(k.configuration.brokers, k.configuration.ConsumerGroup, k.kafkaConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create consumer")
	}

	k.Logger.DebugWith("Consumer created", "brokers", k.configuration.brokers)
	return consumerGroup, nil
}

func (k *kafka) createPartitionWorkerAllocator(session sarama.ConsumerGroupSession) (partitionWorkerAllocator, error) {
	switch k.configuration.WorkerAllocationMode {
	case workerAllocationModePool:
		return newPooledWorkerAllocator(k.Logger, k.WorkerAllocator)

	case workerAllocationModeStatic:
		topicPartitionIDs := map[string][]int{}

		// convert int32 -> int
		for topic, partitionIDs := range session.Claims() {
			for _, partitionID := range partitionIDs {
				topicPartitionIDs[topic] = append(topicPartitionIDs[topic], int(partitionID))
			}
		}

		return newStaticWorkerAllocator(k.Logger, k.WorkerAllocator, topicPartitionIDs)

	default:
		return nil, errors.Errorf("Unknown worker allocation mode: %s", k.configuration.WorkerAllocationMode)
	}
}
