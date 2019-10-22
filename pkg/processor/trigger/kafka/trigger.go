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
	configuration  *Configuration
	kafkaConfig    *sarama.Config
	consumerGroup  sarama.ConsumerGroup
	shutdownSignal chan struct{}
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

	newTrigger.Logger.DebugWith("Creating consumer", "brokers", configuration.brokers)

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

	ctx := context.Background()
	err = k.consumerGroup.Consume(ctx, k.configuration.Topics, k)
	if err != nil {
		return errors.Wrap(err, "Failed to join consumer cluster")
	}

	return err
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

func (k *kafka) newKafkaConfig() (*sarama.Config, error) {

	config := sarama.NewConfig()

	config.Consumer.Offsets.Initial = k.configuration.initialOffset

	config.Net.SASL.Enable = k.configuration.SASL.Enable
	config.Net.SASL.User = k.configuration.SASL.User
	config.Net.SASL.Password = k.configuration.SASL.Password
	config.ClientID = k.ID

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

func (k *kafka) Setup(session sarama.ConsumerGroupSession) error {
	k.Logger.InfoWith("Starting consumer session", "claims", session.Claims())
	return nil
}

func (k *kafka) Cleanup(session sarama.ConsumerGroupSession) error {
	k.Logger.InfoWith("Ending consumer session", "claims", session.Claims())
	return nil
}

func (k *kafka) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	workerInstance, err := k.WorkerAllocator.Allocate(0)
	if err != nil {
		return errors.Wrap(err, "Failed to allocate worker for consumer")
	}
	defer k.WorkerAllocator.Release(workerInstance)
	event := Event{}
	for message := range claim.Messages() {
		event.kafkaMessage = message
		k.SubmitEventToWorker(nil, workerInstance, &event) // nolint: errcheck
		session.MarkMessage(message, "")                   // mark message as processed
	}
	return nil
}
