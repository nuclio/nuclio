package natsjetstream

import (
  "context"
	"net/url"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	natsio "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type natsjetstream struct {
	trigger.AbstractTrigger
	configuration    *Configuration
	stop             chan bool
	natsSubscription *natsio.Subscription
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator eventprocessor.Allocator,
	configuration *Configuration,
	restartTriggerChan chan trigger.Trigger) (trigger.Trigger, error) {
	abstractTrigger, err := trigger.NewAbstractTrigger(parentLogger.GetChild(configuration.ID),
		workerAllocator,
		&configuration.Configuration,
		"async",
		"natsjetstream",
		configuration.Name,
		restartTriggerChan)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger := &natsjetstream{
		AbstractTrigger: abstractTrigger,
		configuration:   configuration,
		stop:            make(chan bool),
	}
	newTrigger.AbstractTrigger.Trigger = newTrigger

	err = newTrigger.validateConfiguration()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to validate NATS trigger configuration")
	}

	return newTrigger, nil
}

func (n *natsjetstream) validateConfiguration() error {
	natsURL, err := url.Parse(n.configuration.URL)
	if err != nil {
		return errors.Wrap(err, "Failed to parse NATS URL")
	}

	if natsURL.Scheme != "nats" {
		return errors.New("Invalid URL. Must begin with 'nats://'")
	}

	return nil
}

func (n *natsjetstream) Start(checkpoint functionconfig.Checkpoint) error {
	natsConnection, err := natsio.Connect(n.configuration.URL)
	if err != nil {
		return errors.Wrapf(err, "Can't connect to NATS server %s", n.configuration.URL)
	}

  jetstreamConnection, err := jetstream.New(natsConnection)
	if err != nil {
		return errors.Wrapf(err, "Can't connect to NATS jetstream server %s", n.configuration.URL)
	}

  consumer, err := jetstreamConnection.Consumer(context.TODO(), n.configuration.Stream, n.configuration.Consumer)
	if err != nil {
		return errors.Wrapf(err, "Can't subscribe to stream %q with consumer %q", n.configuration.Stream, n.configuration.Consumer)
	}

	messageChan := make(chan jetstream.Msg, 64)
  _, err = consumer.Consume(func(msg jetstream.Msg) { messageChan <- msg })
	if err != nil {
		return errors.Wrapf(err, "Can't consume from consumer %q", n.configuration.Consumer)
	}
  
	go n.listenForMessages(messageChan)
	return nil
}

func (n *natsjetstream) Stop(force bool) (functionconfig.Checkpoint, error) {
	n.stop <- true
	return nil, n.natsSubscription.Unsubscribe()
}

func (n *natsjetstream) listenForMessages(messageChan chan jetstream.Msg) {
	for {
		select {
		case natsMessage := <-messageChan:

			// submit the event to the worker in the background and continue,
			// as we don't mark anything when processing is done
			go func() {
				event := &Event{
					natsMessage: natsMessage,
				}

				// allocate a worker
				workerInstance, err := n.WorkerAllocator.Allocate(time.Duration(*n.configuration.WorkerAvailabilityTimeoutMilliseconds) * time.Millisecond)
				if err != nil {
          natsMessage.Nak()
					n.UpdateStatistics(false, 1)
					n.Logger.ErrorWith("Failed to allocate worker", "error", err)
					return
				}

				// submit the event to the worker, don't really do anything with response
				_, processErr := n.SubmitEventToWorker(nil, workerInstance, event)
				if processErr != nil {
          natsMessage.Nak()
					n.Logger.ErrorWith("Can't process event", "error", processErr)
				} else {
          if err := natsMessage.Ack(); err != nil {
            n.Logger.ErrorWith("Failed to acknowledge message", "warning", err)
          }
        }

				// release the worker
				n.WorkerAllocator.Release(workerInstance)
			}()

		case <-n.stop:
			return
		}
	}
}

func (n *natsjetstream) GetConfig() map[string]interface{} {
	return common.StructureToMap(n.configuration)
}
