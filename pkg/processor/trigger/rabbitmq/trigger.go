/*
Copyright 2023 The Nuclio Authors.

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

package rabbitmq

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	amqp "github.com/rabbitmq/amqp091-go"
)

type rabbitMq struct {
	trigger.AbstractTrigger
	configuration              *Configuration
	consumerName               string
	brokerConn                 *amqp.Connection
	brokerChannel              *amqp.Channel
	brokerQueue                amqp.Queue
	brokerInputMessagesChannel <-chan amqp.Delivery
	stopChan                   chan struct{}
	connectionErrorChan        chan *amqp.Error
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator eventprocessor.Allocator,
	configuration *Configuration,
	restartTriggerChan chan trigger.Trigger) (trigger.Trigger, error) {

	abstractTrigger, err := trigger.NewAbstractTrigger(parentLogger.GetChild(configuration.ID),
		workerAllocator,
		&configuration.Configuration,
		"async",
		"rabbit-mq",
		configuration.Name,
		restartTriggerChan)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	triggerInstance := rabbitMq{
		AbstractTrigger: abstractTrigger,
		configuration:   configuration,
	}
	triggerInstance.Trigger = &triggerInstance

	return &triggerInstance, nil
}

func (rmq *rabbitMq) Initialize() error {
	var err error
	rmq.consumerName, err = rmq.getConsumerName()
	if err != nil {
		return errors.Wrap(err, "Failed to resolve consumer name")
	}

	rmq.setEmptyParameters()
	return nil
}

func (rmq *rabbitMq) Start(checkpoint functionconfig.Checkpoint) error {
	rmq.Logger.InfoWith("Starting",
		"consumerName", rmq.consumerName,
		"brokerUrl", rmq.configuration.URL)

	rmq.stopChan = make(chan struct{})

	if err := rmq.createBrokerResources(); err != nil {
		return errors.Wrap(err, "Failed to create broker resources")
	}

	// start listening for published messages
	go rmq.handleBrokerMessages()

	return nil
}

func (rmq *rabbitMq) Stop(force bool) (functionconfig.Checkpoint, error) {

	// stop listening for messages
	close(rmq.stopChan)

	// close broker
	if err := rmq.brokerChannel.Close(); err != nil {
		rmq.Logger.WarnWith("Failed to close broker channel", "err", err.Error())
	}

	close(rmq.connectionErrorChan)
	return nil, nil
}

func (rmq *rabbitMq) GetConfig() map[string]interface{} {
	return common.StructureToMap(rmq.configuration)
}

func (rmq *rabbitMq) setEmptyParameters() {
	if rmq.configuration.QueueName == "" {
		rmq.configuration.QueueName = fmt.Sprintf("nuclio-%s-%s",
			rmq.configuration.RuntimeConfiguration.Meta.Namespace,
			rmq.configuration.RuntimeConfiguration.Meta.Name)
	}
}

func (rmq *rabbitMq) createBrokerResources() error {
	rmq.Logger.InfoWith("Creating broker resources",
		"brokerUrl", rmq.configuration.URL,
		"exchangeName", rmq.configuration.ExchangeName,
		"queueName", rmq.configuration.QueueName,
		"topics", rmq.configuration.Topics)

	// create connection
	if err := rmq.connect(); err != nil {
		return errors.Wrap(err, "Failed to connect to broker")
	}

	// create topics (and exchange, etc)
	if err := rmq.createTopics(); err != nil {
		return errors.Wrap(err, "Failed to create topics")
	}

	// ensure the queue exists before consuming to avoid "delivery not initialized" ack errors
	if err := rmq.ensureQueueExists(); err != nil {
		return errors.Wrap(err, "Queue does not exist")
	}

	// consume from queue
	if err := rmq.consume(); err != nil {
		return errors.Wrap(err, "Failed to consume messages")
	}

	return nil
}

func (rmq *rabbitMq) getConnectionConfig() *amqp.Config {
	config := amqp.Config{Properties: amqp.NewConnectionProperties()}

	if rmq.configuration.Username != "" && rmq.configuration.Password != "" {
		config.SASL = []amqp.Authentication{
			&amqp.PlainAuth{
				Username: rmq.configuration.Username,
				Password: rmq.configuration.Password,
			},
		}
	}

	connectionName := rmq.FunctionName + "-" + rmq.ID

	// when running processor locally, there might be no function name.
	connectionName = strings.TrimLeft(connectionName, "-")
	if !strings.HasSuffix(connectionName, "nuclio-") {
		connectionName = "nuclio-" + connectionName
	}

	config.Properties.SetClientConnectionName(connectionName)
	return &config
}

func (rmq *rabbitMq) handleBrokerMessages() {
	for {
		select {
		case err := <-rmq.connectionErrorChan:

			// TODO: do not leave function dead in the water, perhaps restart the trigger?
			if handleErr := rmq.handleConnectionError(err); handleErr != nil {
				rmq.Logger.ErrorWith("Failed to handle connection error", "err", handleErr)
				panic(handleErr)
			}
			rmq.Logger.Info("Successfully handled connection error")
		case <-rmq.stopChan:
			rmq.Logger.DebugWith("Stopping consumption from queue", "queueName", rmq.configuration.QueueName)
			return
		case message := <-rmq.brokerInputMessagesChannel:
			rmq.processMessage(&message)
		}
	}
}

func (rmq *rabbitMq) reconnect() error {
	rmq.Logger.DebugWith("Reconnecting to broker",
		"brokerUrl", rmq.configuration.URL,
		"duration", rmq.configuration.reconnectDuration.String(),
		"interval", rmq.configuration.reconnectInterval.Seconds())
	timeStart := time.Now()
	if err := common.RetryUntilSuccessful(rmq.configuration.reconnectDuration,
		rmq.configuration.reconnectInterval,
		func() bool {
			if err := rmq.connect(); err != nil {
				rmq.Logger.WarnWith("Failed to connect to broker, retrying",
					"interval", rmq.configuration.reconnectInterval.String(),
					"timeLeft", (rmq.configuration.reconnectDuration - time.Since(timeStart)).String(),
					"err", err.Error())
				return false
			}
			return true
		}); err != nil {
		return errors.Wrap(err, "Failed to reconnect to broker")
	}

	rmq.Logger.DebugWith("Reconnected to broker",
		"consumerName", rmq.consumerName,
		"brokerUrl", rmq.configuration.URL)
	return nil
}

func (rmq *rabbitMq) connect() error {
	var err error

	rmq.brokerConn, err = amqp.DialConfig(rmq.configuration.URL, *rmq.getConnectionConfig())
	if err != nil {
		return errors.Wrap(err, "Failed to create connection to broker")
	}

	rmq.connectionErrorChan = make(chan *amqp.Error)
	rmq.brokerConn.NotifyClose(rmq.connectionErrorChan)
	rmq.Logger.DebugWith("Connected to broker", "brokerUrl", rmq.configuration.URL)

	rmq.brokerChannel, err = rmq.brokerConn.Channel()
	if err != nil {
		return errors.Wrap(err, "Failed to create channel")
	}
	rmq.Logger.DebugWith("Created broker channel")
	return nil
}

func (rmq *rabbitMq) createTopics() error {
	var err error
	if len(rmq.configuration.Topics) == 0 {
		return nil
	}

	// create exchange and queue only if user provided topics, else assuming the user did all the necessary configuration
	// to support listening on the provided exchange and queue
	// TODO: move to ui and add feature flag

	if rmq.configuration.PrefetchCount != 0 {
		if err := rmq.brokerChannel.Qos(rmq.configuration.PrefetchCount, 0, true); err != nil {
			return errors.Wrap(err, "Failed to setup prefetch on channel")
		}
	}

	// create the exchange
	if err := rmq.brokerChannel.ExchangeDeclare(rmq.configuration.ExchangeName,
		"topic",
		rmq.configuration.DurableExchange,
		false,
		false,
		false,
		nil); err != nil {
		return errors.Wrap(err, "Failed to declare exchange")
	}
	rmq.Logger.DebugWith("Declared exchange", "exchangeName", rmq.configuration.ExchangeName)

	rmq.brokerQueue, err = rmq.brokerChannel.QueueDeclare(
		rmq.configuration.QueueName,    // queue name (account  + function name)
		rmq.configuration.DurableQueue, // durable  TBD: change to true if/when we bind to persistent storage
		false,                          // delete when unused
		false,                          // exclusive
		false,                          // no-wait
		nil,                            // arguments
	)
	if err != nil {
		return errors.Wrap(err, "Failed to declare queue")
	}
	rmq.Logger.DebugWith("Declared queue", "queueName", rmq.brokerQueue.Name)

	for _, topic := range rmq.configuration.Topics {
		if err := rmq.brokerChannel.QueueBind(
			rmq.brokerQueue.Name,           // queue name
			topic,                          // routing key
			rmq.configuration.ExchangeName, // exchange
			false,
			nil); err != nil {
			return errors.Wrap(err, "Failed to bind to queue")
		}
		rmq.Logger.DebugWith("Bound queue to topic",
			"queueName", rmq.brokerQueue.Name,
			"topic", topic,
			"exchangeName", rmq.configuration.ExchangeName)

	}
	return nil
}

// ensureQueueExists verifies that the configured queue exists before starting consumption.
// This prevents "delivery not initialized" ack errors when the queue does not exist.
func (rmq *rabbitMq) ensureQueueExists() error {
	checkChannel, err := rmq.brokerConn.Channel()
	if err != nil {
		return errors.Wrap(err, "Failed to create channel for queue check")
	}
	defer checkChannel.Close()
	// Passive declare only checks that the queue exists; it does not create or redeclare the queue with these params.
	_, err = checkChannel.QueueDeclarePassive(
		rmq.configuration.QueueName,
		false, // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return errors.Wrapf(err, "Queue %q does not exist", rmq.configuration.QueueName)
	}
	return nil
}

func (rmq *rabbitMq) consume() error {
	var err error

	rmq.brokerInputMessagesChannel, err = rmq.brokerChannel.Consume(
		rmq.configuration.QueueName,
		rmq.consumerName,
		false, /* auto-ack */
		false, /* exclusive */
		false, /* no-local */
		true,  /* no-wait */
		nil,   /* args */
	)
	if err != nil {
		return errors.Wrap(err, "Failed to start consuming messages")
	}

	rmq.Logger.DebugWith("Starting consumption from queue", "queueName", rmq.configuration.QueueName)
	return nil
}

func (rmq *rabbitMq) handleConnectionError(handleErr *amqp.Error) error {
	rmq.Logger.WarnWith("Broker connection closed",
		"err", handleErr.Error(),
		"serverErr", handleErr.Server,
		"recover", handleErr.Recover,
		"reason", handleErr.Reason,
		"code", handleErr.Code)

	// best effort closing broker connection and channel
	if err := rmq.brokerConn.Close(); err != nil {
		rmq.Logger.WarnWith("Failed to close broker connection", "err", err.Error())
	}
	if err := rmq.brokerChannel.Close(); err != nil {
		rmq.Logger.WarnWith("Failed to close broker channel", "err", err.Error())
	}

	if err := rmq.reconnect(); err != nil {
		return errors.Wrap(err, "Failed to reconnect to broker")
	}

	// start message consumption again
	if err := rmq.consume(); err != nil {
		return errors.Wrap(err, "Failed to start consuming messages")
	}
	return nil
}

func (rmq *rabbitMq) processMessage(message *amqp.Delivery) {

	event := rmq.createEventFromMessage(message)

	// submit to worker
	response, submitError, processError := rmq.AllocateWorkerAndSubmitEvent(event,
		nil, time.Duration(*rmq.configuration.WorkerAvailabilityTimeoutMilliseconds)*time.Millisecond)

	// if submission failed, nack the message so it can be redelivered
	if submitError != nil {
		rmq.Logger.DebugWith("Failed to submit event, message will be nacked and requeued",
			"err", submitError.Error())

		if err := message.Nack(false, true); err != nil {
			rmq.Logger.WarnWith("Failed to nack message",
				"error", err.Error())
		}
		return
	}

	if processError == nil && response.GetStatusCode() >= 400 {
		processError = errors.New(fmt.Sprintf("Function returned error status code: %d", response.GetStatusCode()))
	}

	// if processing failed, we can choose to nack or ack based on the configuration
	if processError != nil {
		rmq.Logger.DebugWith("Event processing failed, handling according to ACK configuration",
			"err", processError.Error())
		rmq.ackOnProcessError(message)
		return
	}

	// ack the message if processing succeeded
	if err := message.Ack(false); err != nil {
		rmq.Logger.WarnWith("Failed to ack message",
			"error", err.Error())
	}
}

func (rmq *rabbitMq) createEventFromMessage(message *amqp.Delivery) *Event {
	event := &Event{
		message: message,
	}
	event.SetID(nuclio.ID(message.MessageId))
	return event
}

func (rmq *rabbitMq) ackOnProcessError(message *amqp.Delivery) {
	switch rmq.configuration.OnError {
	case OnProcessErrorAck:
		if err := message.Ack(false); err != nil {
			rmq.Logger.WarnWith("Failed to ack message",
				"error", err.Error())
		}
	default:
		if err := message.Nack(false, rmq.configuration.RequeueOnError); err != nil {
			rmq.Logger.WarnWith("Failed to nack message",
				"error", err.Error())
		}
	}
}

func (rmq *rabbitMq) getConsumerName() (string, error) {
	var consumerName string
	var err error
	if common.IsInKubernetesCluster() {

		// in k8s, use the pod name and trigger id as the consumer name
		consumerName = os.Getenv("HOSTNAME") + rmq.ID
	} else if common.RunningInContainer() {
		if consumerName, err = common.RunningContainerHostname(); err != nil {
			return "", errors.Wrap(err, "Failed to get container hostname")
		}
	}

	// empty to let the client generate a random name
	return consumerName, nil
}
