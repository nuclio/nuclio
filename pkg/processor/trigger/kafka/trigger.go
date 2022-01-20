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
	"crypto/tls"
	"crypto/x509"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/trigger/kafka/scram"
	"github.com/nuclio/nuclio/pkg/processor/trigger/kafka/tokenprovider/oauth"
	"github.com/nuclio/nuclio/pkg/processor/util/partitionworker"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/Shopify/sarama"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/rcrowley/go-metrics"
)

type submittedEvent struct {
	event  Event
	worker *worker.Worker
	done   chan error
}

type kafka struct {
	trigger.AbstractTrigger
	configuration            *Configuration
	kafkaConfig              *sarama.Config
	consumerGroup            sarama.ConsumerGroup
	shutdownSignal           chan struct{}
	stopConsumptionChan      chan struct{}
	partitionWorkerAllocator partitionworker.Allocator
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {
	var err error

	// first - disable sarama metrics, as they leak memory
	metrics.UseNilMetrics = true

	loggerInstance := parentLogger.GetChild(configuration.ID)

	sarama.Logger = NewSaramaLogger(loggerInstance)

	newTrigger := &kafka{
		configuration:       configuration,
		stopConsumptionChan: make(chan struct{}, 1),
	}

	newTrigger.AbstractTrigger, err = trigger.NewAbstractTrigger(loggerInstance,
		workerAllocator,
		&configuration.Configuration,
		"async",
		"kafka-cluster",
		configuration.Name)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger.Logger.DebugWith("Creating consumer",
		"brokers", configuration.brokers,
		"workerAllocationMode", configuration.WorkerAllocationMode,
		"sessionTimeout", configuration.sessionTimeout,
		"heartbeatInterval", configuration.heartbeatInterval,
		"rebalanceTimeout", configuration.rebalanceTimeout,
		"rebalanceTimeout", configuration.rebalanceTimeout,
		"retryBackoff", configuration.retryBackoff,
		"maxWaitTime", configuration.maxWaitTime,
		"rebalanceRetryMax", configuration.RebalanceRetryMax,
		"fetchMin", configuration.FetchMin,
		"fetchDefault", configuration.FetchDefault,
		"fetchMax", configuration.FetchMax,
		"channelBufferSize", configuration.ChannelBufferSize,
		"maxWaitHandlerDuringRebalance", configuration.maxWaitHandlerDuringRebalance,
		"logLevel", configuration.LogLevel)

	newTrigger.kafkaConfig, err = newTrigger.newKafkaConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create configuration")
	}

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
				k.Logger.WarnWith("Failed to consume from group, waiting before retrying", "err", errors.GetErrorStackString(err, 10))

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
		"memberID", session.MemberID(),
		"workersAvailable", k.WorkerAllocator.GetNumWorkersAvailable())

	k.partitionWorkerAllocator, err = k.createPartitionWorkerAllocator(session)
	if err != nil {
		return errors.Wrap(err, "Failed to create partition worker allocator")
	}

	return nil
}

func (k *kafka) Cleanup(session sarama.ConsumerGroupSession) error {
	err := k.partitionWorkerAllocator.Stop()
	if err != nil {
		return errors.Wrap(err, "Failed to stop partition worker allocator")
	}

	k.Logger.InfoWith("Ending consumer session",
		"claims", session.Claims(),
		"memberID", session.MemberID(),
		"workersAvailable", k.WorkerAllocator.GetNumWorkersAvailable())

	return nil
}

func (k *kafka) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	var submitError error

	// cleared when the consumption should stop
	consumeMessages := true

	submittedEventInstance := submittedEvent{
		done: make(chan error),
	}

	submittedEventChan := make(chan *submittedEvent)

	// submit the events in a goroutine so that we can unblock immediately
	go k.eventSubmitter(claim, submittedEventChan)

	ackWindowSize := int64(k.configuration.ackWindowSize)
	if k.configuration.ackWindowSize > 0 {
		k.Logger.DebugWith("Starting claim consumption with ack window",
			"partition", claim.Partition(),
			"ackWindowSize", ackWindowSize)
	}

	// the exit condition is that (a) the Messages() channel was closed and (b) we got a signal telling us
	// to stop consumption
	for message := range claim.Messages() {
		if !consumeMessages {
			k.Logger.DebugWith("Stopping message consumption", "partition", claim.Partition())

			break
		}

		// allocate a worker for this topic/partition
		workerInstance, cookie, err := k.partitionWorkerAllocator.AllocateWorker(claim.Topic(),
			int(claim.Partition()),
			nil)
		if err != nil {
			return errors.Wrap(err, "Failed to allocate worker")
		}

		submittedEventInstance.event.kafkaMessage = message
		submittedEventInstance.worker = workerInstance

		// handle in the goroutine so we don't block
		submittedEventChan <- &submittedEventInstance

		// wait for handling done or indication to stop
		select {
		case err := <-submittedEventInstance.done:

			// we successfully submitted the message to the handler. mark it
			if err == nil {
				session.MarkOffset(
					message.Topic,
					message.Partition,
					message.Offset+1-ackWindowSize,
					"",
				)
			}

		case <-claim.StopConsuming():
			k.Logger.DebugWith("Got signal to stop consumption",
				"wait", k.configuration.maxWaitHandlerDuringRebalance,
				"partition", claim.Partition())

			// don't consume any more messages
			consumeMessages = false

			// wait a bit more for event to process
			select {
			case <-submittedEventInstance.done:
				k.Logger.DebugWith("Handler done, rebalancing will commence")

			case <-time.After(k.configuration.maxWaitHandlerDuringRebalance):
				k.Logger.DebugWith("Timed out waiting for handler to complete", "partition", claim.Partition())

				// mark this as a failure, metric-wise
				k.UpdateStatistics(false)

				// restart the worker, and having failed that shut down
				if err := k.cancelEventHandling(workerInstance, claim); err != nil {
					k.Logger.DebugWith("Failed to cancel event handling",
						"err", err.Error(),
						"partition", claim.Partition())

					panic("Failed to cancel event handling")
				}
			}
		}

		// release the worker from whence it came
		err = k.partitionWorkerAllocator.ReleaseWorker(cookie, workerInstance)
		if err != nil {
			return errors.Wrap(err, "Failed to release worker")
		}
	}

	k.Logger.DebugWith("Claim consumption stopped", "partition", claim.Partition())

	// shut down the event submitter
	close(submittedEventChan)

	return submitError
}

func (k *kafka) eventSubmitter(claim sarama.ConsumerGroupClaim, submittedEventChan chan *submittedEvent) {
	k.Logger.DebugWith("Event submitter started",
		"topic", claim.Topic(),
		"partition", claim.Partition())

	// while there are events to submit, submit them to the given worker
	for submittedEvent := range submittedEventChan {

		// submit the event to the worker
		_, processErr := k.SubmitEventToWorker(nil, submittedEvent.worker, &submittedEvent.event) // nolint: errcheck
		if processErr != nil {
			k.Logger.DebugWith("Process error",
				"partition", submittedEvent.event.kafkaMessage.Partition,
				"err", processErr)
		}

		// indicate that we're done
		submittedEvent.done <- processErr
	}

	k.Logger.DebugWith("Event submitter stopped",
		"topic", claim.Topic(),
		"partition", claim.Partition())
}

func (k *kafka) cancelEventHandling(workerInstance *worker.Worker, claim sarama.ConsumerGroupClaim) error {
	if workerInstance.SupportsRestart() {
		return workerInstance.Restart()
	}

	return errors.New("Worker doesn't support restart")
}

func (k *kafka) newKafkaConfig() (*sarama.Config, error) {
	var err error
	config := sarama.NewConfig()

	config.ClientID = k.ID
	config.Consumer.Offsets.Initial = k.configuration.initialOffset
	config.Consumer.Offsets.AutoCommit.Enable = true
	config.Consumer.Group.Session.Timeout = k.configuration.sessionTimeout
	config.Consumer.Group.Heartbeat.Interval = k.configuration.heartbeatInterval
	config.Consumer.Group.Rebalance.Timeout = k.configuration.rebalanceTimeout
	config.Consumer.Group.Rebalance.Retry.Max = k.configuration.RebalanceRetryMax
	config.Consumer.Group.Rebalance.Retry.Backoff = k.configuration.rebalanceRetryBackoff
	config.Consumer.Group.Rebalance.Strategy = k.configuration.balanceStrategy
	config.Consumer.Retry.Backoff = k.configuration.retryBackoff
	config.Consumer.Fetch.Min = int32(k.configuration.FetchMin)
	config.Consumer.Fetch.Default = int32(k.configuration.FetchDefault)
	config.Consumer.Fetch.Max = int32(k.configuration.FetchMax)
	config.Consumer.MaxWaitTime = k.configuration.maxWaitTime
	config.Consumer.MaxProcessingTime = k.configuration.maxProcessingTime
	config.ChannelBufferSize = k.configuration.ChannelBufferSize
	config.LogLevel = k.configuration.LogLevel

	// configure TLS if applicable
	config.Net.TLS.Enable = k.configuration.CACert != "" || k.configuration.TLS.Enable
	if config.Net.TLS.Enable {
		k.Logger.DebugWith("Enabling TLS",
			"calen", len(k.configuration.CACert))
		config.Net.TLS.Config = &tls.Config{
			InsecureSkipVerify: k.configuration.TLS.InsecureSkipVerify,
		}

		if k.configuration.CACert != "" {
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM([]byte(k.configuration.CACert))
			config.Net.TLS.Config.RootCAs = caCertPool

			if k.configuration.AccessKey != "" && k.configuration.AccessCertificate != "" {
				k.Logger.DebugWith("Configuring cert authentication",
					"keylen", len(k.configuration.AccessKey),
					"certlen", len(k.configuration.AccessCertificate))

				keypair, err := tls.X509KeyPair([]byte(k.configuration.AccessCertificate), []byte(k.configuration.AccessKey))
				if err != nil {
					return nil, errors.Wrap(err, "Failed to create X.509 key pair")
				}

				config.Net.TLS.Config.Certificates = []tls.Certificate{keypair}
			}
		}
	}

	// configure SASL if applicable
	if k.configuration.SASL.Enable {
		k.Logger.DebugWith("Configuring SASL authentication",
			"username", k.configuration.SASL.User,
			"mechanism", k.configuration.SASL.Mechanism)

		config.Net.SASL.Enable = true
		config.Net.SASL.User = k.configuration.SASL.User
		config.Net.SASL.Password = k.configuration.SASL.Password
		config.Net.SASL.Mechanism = sarama.SASLMechanism(k.configuration.SASL.Mechanism)
		config.Net.SASL.Handshake = k.configuration.SASL.Handshake
		config.Net.SASL.SCRAMClientGeneratorFunc = k.resolveSCRAMClientGeneratorFunc(config.Net.SASL.Mechanism)

		// per mechanism configuration
		if config.Net.SASL.Mechanism == sarama.SASLTypeOAuth {
			config.Net.SASL.TokenProvider = oauth.NewTokenProvider(context.TODO(),
				k.configuration.SASL.OAuth.ClientID,
				k.configuration.SASL.OAuth.ClientSecret,
				k.configuration.SASL.OAuth.TokenURL,
				k.configuration.SASL.OAuth.Scopes)
		}
	}

	// V0_10_2_0 is the minimum required for sarama's consumer groups implementation.
	// Therefore, we do not support anything older that this version.
	// Update: increasing version to V0_11_0_0 because it's the minimum version that is required
	// to support kafka headers.
	version := sarama.V0_11_0_0

	if k.configuration.Version != "" {
		version, err = sarama.ParseKafkaVersion(k.configuration.Version)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse kafka version - %s", k.configuration.Version)
		}
		if !version.IsAtLeast(sarama.V0_11_0_0) {
			return nil, errors.Errorf("Minimum version of 0.11.0 is required, got - %s", version.String())
		}
	}

	config.Version = version

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

func (k *kafka) createPartitionWorkerAllocator(session sarama.ConsumerGroupSession) (partitionworker.Allocator, error) {
	switch k.configuration.WorkerAllocationMode {
	case partitionworker.AllocationModePool:
		return partitionworker.NewPooledWorkerAllocator(k.Logger, k.WorkerAllocator)

	case partitionworker.AllocationModeStatic:
		topicPartitionIDs := map[string][]int{}

		// convert int32 -> int
		for topic, partitionIDs := range session.Claims() {
			for _, partitionID := range partitionIDs {
				topicPartitionIDs[topic] = append(topicPartitionIDs[topic], int(partitionID))
			}
		}

		return partitionworker.NewStaticWorkerAllocator(k.Logger, k.WorkerAllocator, topicPartitionIDs)

	default:
		return nil, errors.Errorf("Unknown worker allocation mode: %s", k.configuration.WorkerAllocationMode)
	}
}

func (k *kafka) resolveSCRAMClientGeneratorFunc(mechanism sarama.SASLMechanism) func() sarama.SCRAMClient {
	switch mechanism {
	case sarama.SASLTypeSCRAMSHA256, sarama.SASLTypeSCRAMSHA512:
		return func() sarama.SCRAMClient { return scram.NewClient(mechanism) }
	default:
		return nil
	}
}
