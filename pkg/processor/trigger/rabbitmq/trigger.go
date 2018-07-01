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

package rabbitmq

import (
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/logger"
	"github.com/streadway/amqp"
)

type rabbitMq struct {
	trigger.AbstractTrigger
	event                      Event
	configuration              *Configuration
	brokerConn                 *amqp.Connection
	brokerChannel              *amqp.Channel
	brokerQueue                amqp.Queue
	brokerInputMessagesChannel <-chan amqp.Delivery
	worker                     *worker.Worker
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {

	newTrigger := rabbitMq{
		AbstractTrigger: trigger.AbstractTrigger{
			ID:              configuration.ID,
			Logger:          parentLogger.GetChild(configuration.ID),
			WorkerAllocator: workerAllocator,
			Class:           "async",
			Kind:            "rabbitMq",
		},
		configuration: configuration,
	}

	return &newTrigger, nil
}

func (rmq *rabbitMq) Start(checkpoint functionconfig.Checkpoint) error {
	var err error

	rmq.Logger.InfoWith("Starting", "brokerUrl", rmq.configuration.URL)

	// get a worker, we'll be using this one always
	rmq.worker, err = rmq.WorkerAllocator.Allocate(10 * time.Second)
	if err != nil {
		return errors.Wrap(err, "Failed to allocate worker")
	}

	rmq.setEmptyParameters()

	if err := rmq.createBrokerResources(); err != nil {
		return errors.Wrap(err, "Failed to create broker resources")
	}

	// start listening for published messages
	go rmq.handleBrokerMessages()

	return nil
}

func (rmq *rabbitMq) Stop(force bool) (functionconfig.Checkpoint, error) {

	// TODO
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

	if len(rmq.configuration.Topics) == 0 {
		rmq.configuration.Topics = []string{"*"}
	}
}

func (rmq *rabbitMq) createBrokerResources() error {
	var err error

	rmq.Logger.InfoWith("Creating broker resources",
		"brokerUrl", rmq.configuration.URL,
		"exchangeName", rmq.configuration.ExchangeName,
		"queueName", rmq.configuration.QueueName,
		"topics", rmq.configuration.Topics)

	rmq.brokerConn, err = amqp.Dial(rmq.configuration.URL)
	if err != nil {
		return errors.Wrap(err, "Failed to create connection to broker")
	}

	rmq.Logger.DebugWith("Connected to broker", "brokerUrl", rmq.configuration.URL)

	rmq.brokerChannel, err = rmq.brokerConn.Channel()
	if err != nil {
		return errors.Wrap(err, "Failed to create channel")
	}

	rmq.Logger.DebugWith("Created broker channel")

	// create the exchange
	err = rmq.brokerChannel.ExchangeDeclare(rmq.configuration.ExchangeName,
		"topic",
		false,
		false,
		false,
		false,
		nil)
	if err != nil {
		return errors.Wrap(err, "Failed to declare exchange")
	}

	rmq.Logger.DebugWith("Declared exchange", "exchangeName", rmq.configuration.ExchangeName)

	rmq.brokerQueue, err = rmq.brokerChannel.QueueDeclare(
		rmq.configuration.QueueName, // queue name (account  + function name)
		false, // durable  TBD: change to true if/when we bind to persistent storage
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return errors.Wrap(err, "Failed to declare queue")
	}

	rmq.Logger.DebugWith("Declared queue", "queueName", rmq.brokerQueue.Name)

	for _, topic := range rmq.configuration.Topics {
		err = rmq.brokerChannel.QueueBind(
			rmq.brokerQueue.Name, // queue name
			topic,                // routing key
			rmq.configuration.ExchangeName, // exchange
			false,
			nil)
		if err != nil {
			return errors.Wrap(err, "Failed to bind to queue")
		}

		rmq.Logger.DebugWith("Bound queue to topic",
			"queueName", rmq.brokerQueue.Name,
			"topic", topic,
			"exchangeName", rmq.configuration.ExchangeName)

	}

	rmq.brokerInputMessagesChannel, err = rmq.brokerChannel.Consume(
		rmq.brokerQueue.Name, // queue
		"",                   // consumer
		false,                // auto-ack
		false,                // exclusive
		false,                // no-local
		true,                 // no-wait
		nil,                  // args
	)
	if err != nil {
		return errors.Wrap(err, "Failed to start consuming messages")
	}

	rmq.Logger.DebugWith("Starting consumption from queue", "queueName", rmq.brokerQueue.Name)

	return nil
}

func (rmq *rabbitMq) handleBrokerMessages() {
	for message := range rmq.brokerInputMessagesChannel {

		// bind to delivery
		rmq.event.message = &message

		// submit to worker
		_, submitError, _ := rmq.AllocateWorkerAndSubmitEvent(&rmq.event, nil, 10*time.Second)

		// ack the message if we didn't fail to submit
		if submitError == nil {
			message.Ack(false) // nolint: errcheck
		} else {
			rmq.Logger.WarnWith("Failed to submit to worker", "err", submitError)
		}
	}
}
