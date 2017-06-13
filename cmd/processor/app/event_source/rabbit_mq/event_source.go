package rabbit_mq

import (
	"time"

	"github.com/streadway/amqp"

	"github.com/nuclio/nuclio/cmd/processor/app/event_source"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"
)

type rabbit_mq struct {
	event_source.AbstractEventSource
	event                      Event
	configuration              *Configuration
	brokerConn                 *amqp.Connection
	brokerChannel              *amqp.Channel
	brokerQueue                amqp.Queue
	brokerInputMessagesChannel <-chan amqp.Delivery
	worker                     *worker.Worker
}

func NewEventSource(logger logger.Logger,
	workerAllocator worker.WorkerAllocator,
	configuration *Configuration) (event_source.EventSource, error) {

	newEventSource := rabbit_mq{
		AbstractEventSource: event_source.AbstractEventSource{
			Logger:          logger.GetChild("rabbit_mq"),
			WorkerAllocator: workerAllocator,
			Class:           "async",
			Kind:            "rabbit_mq",
		},
		configuration: configuration,
	}

	return &newEventSource, nil
}

func (rmq *rabbit_mq) Start(checkpoint event_source.Checkpoint) error {
	var err error

	rmq.Logger.With(logger.Fields{
		"brokerUrl": rmq.configuration.BrokerUrl,
	}).Info("Starting")

	// get a worker, we'll be using this one always
	rmq.worker, err = rmq.WorkerAllocator.Allocate(10 * time.Second)
	if err != nil {
		return rmq.Logger.Report(err, "Failed to allocate worker")
	}

	if err := rmq.createBrokerResources(); err != nil {
		return rmq.Logger.Report(err, "Failed to create broker resources")
	}

	// start listening for published messages
	go rmq.handleBrokerMessages()

	return nil
}

func (rmq *rabbit_mq) Stop(force bool) (event_source.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (rmq *rabbit_mq) createBrokerResources() error {
	var err error

	rmq.brokerConn, err = amqp.Dial(rmq.configuration.BrokerUrl)
	if err != nil {
		return rmq.Logger.Report(err, "Failed to create connection to broker")
	}

	rmq.brokerChannel, err = rmq.brokerConn.Channel()
	if err != nil {
		return rmq.Logger.Report(err, "Failed to create channel")
	}

	rmq.brokerQueue, err = rmq.brokerChannel.QueueDeclare(
		"foo", // queue name (account  + function name)
		false, // durable  TBD: change to true if/when we bind to persistent storage
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return rmq.Logger.Report(err, "Failed to create queue")
	}

	err = rmq.brokerChannel.QueueBind(
		rmq.brokerQueue.Name, // queue name
		"foo",                // routing key
		rmq.configuration.BrokerExchangeName, // exchange
		false,
		nil)
	if err != nil {
		return rmq.Logger.Report(err, "Failed to bind to queue")
	}

	rmq.brokerInputMessagesChannel, err = rmq.brokerChannel.Consume(
		rmq.brokerQueue.Name, // queue
		"",                   // consumer
		false,                // auto-ack
		false,                // exclusive
		false,                // no-local
		false,                // no-wait
		nil,                  // args
	)
	if err != nil {
		return rmq.Logger.Report(err, "Failed to start consuming messages")
	}

	return nil
}

func (rmq *rabbit_mq) handleBrokerMessages() {
	for {
		select {
		case message := <-rmq.brokerInputMessagesChannel:

			// bind to delivery
			rmq.event.message = &message

			// submit to worker
			_, submitError, processError := rmq.SubmitEventToWorker(&rmq.event, 10*time.Second)

			// TODO: do something with response and process error?
			rmq.Logger.With(logger.Fields{
				"processError": processError,
			}).Debug("Processed message")

			// ack the message if we didn't fail to submit
			if submitError == nil {
				message.Ack(false)
			} else {
				rmq.Logger.Report(submitError, "Failed to submit to worker")
			}
		}
	}
}
