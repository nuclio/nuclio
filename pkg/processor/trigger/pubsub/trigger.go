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

package pubsub

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	pubsubClient "cloud.google.com/go/pubsub"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/rs/xid"
)

type pubsub struct {
	trigger.AbstractTrigger
	configuration *Configuration
	stop          chan bool
	client        *pubsubClient.Client
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {
	abstractTrigger, err := trigger.NewAbstractTrigger(parentLogger.GetChild(configuration.ID),
		workerAllocator,
		&configuration.Configuration,
		"async",
		"pubsub",
		configuration.Name)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger := &pubsub{
		AbstractTrigger: abstractTrigger,
		configuration:   configuration,
		stop:            make(chan bool),
	}

	return newTrigger, nil
}

func (p *pubsub) Start(checkpoint functionconfig.Checkpoint) error {
	var err error

	p.Logger.InfoWith("Starting",
		"subscriptions", p.configuration.Subscriptions,
		"projectID", p.configuration.ProjectID,
	)

	// ensure application credentials (aka service-account)
	if err := p.setAndValidateGoogleApplicationCredentials(); err != nil {
		return err
	}

	// pubsub client consumes namespace/project string to be created
	p.client, err = pubsubClient.NewClient(context.TODO(), p.configuration.ProjectID)
	if err != nil {
		return errors.Wrapf(err, "Can't connect to pubsub project")
	}

	p.Logger.DebugWith("Created client")

	for _, subscription := range p.configuration.Subscriptions {
		subscription := subscription

		go func() {
			if err := p.receiveFromSubscription(&subscription); err != nil {
				p.Logger.WarnWith("Failed to create subscription",
					"err", errors.GetErrorStackString(err, 10),
					"subscription", subscription)
			}
		}()
	}

	return nil
}

func (p *pubsub) Stop(force bool) (functionconfig.Checkpoint, error) {

	// TODO:
	// err := p.client.Close()
	// return nil, err
	return nil, nil
}

func (p *pubsub) GetConfig() map[string]interface{} {
	return common.StructureToMap(p.configuration)
}

func (p *pubsub) receiveFromSubscription(subscriptionConfig *Subscription) error {
	ctx := context.TODO()

	p.Logger.DebugWith("Receiving from subscription", "subscription", subscriptionConfig)

	// get subscription id
	subscriptionID := p.getSubscriptionID(subscriptionConfig)

	// get ack timeout
	ackDeadline, err := p.getAckDeadline(subscriptionConfig)
	if err != nil {
		return errors.Wrap(err, "Failed to parse ack deadline")
	}

	subscription, err := p.createOrUseSubscription(ctx, subscriptionID, ackDeadline, subscriptionConfig)
	if err != nil {
		return errors.Wrapf(err, "Failed to create or use subscription %s", subscriptionID)
	}

	// create a channel of events
	eventsChan := make(chan *Event, subscriptionConfig.MaxNumWorkers)
	for eventIdx := 0; eventIdx < subscriptionConfig.MaxNumWorkers; eventIdx++ {
		eventsChan <- &Event{
			topic: subscriptionConfig.Topic,
		}
	}

	p.Logger.DebugWith("Reading from subscription",
		"subscription.ReceiveSettings.NumGoroutines", subscription.ReceiveSettings.NumGoroutines)

	// listen to subscribed topic messages
	err = subscription.Receive(ctx, func(ctx context.Context, message *pubsubClient.Message) {

		// get an event
		event := <-eventsChan

		// set the message
		event.message = message

		// process the event, don't really do anything with response
		_, submitError, processError := p.AllocateWorkerAndSubmitEvent(event, p.Logger, 10*time.Second)
		if submitError != nil {
			p.Logger.ErrorWith("Can't submit event", "error", submitError)

			message.Nack() // necessary to call on fail
			eventsChan <- event
			return
		}
		if processError != nil {
			p.Logger.ErrorWith("Can't process event", "error", processError)

			message.Nack()
			eventsChan <- event
			return
		}

		message.Ack()

		// return event to pool
		eventsChan <- event
	})

	if err != context.Canceled {
		return errors.Wrapf(err, "Message receiver cancelled")
	}

	return nil
}

func (p *pubsub) getSubscriptionID(subscriptionConfig *Subscription) string {
	subscriptionID := subscriptionConfig.IDPrefix + subscriptionConfig.Topic

	// if multiple replicas share this subscription it must be named the same
	if subscriptionConfig.Shared {
		return subscriptionID
	}

	// if it's not shared, we must add a unique variable
	return fmt.Sprintf("%s-%s", subscriptionID, xid.New().String())
}

func (p *pubsub) getAckDeadline(subscriptionConfig *Subscription) (time.Duration, error) {
	var ackDeadlineString string

	switch {
	case subscriptionConfig.AckDeadline != "":
		ackDeadlineString = subscriptionConfig.AckDeadline
	case p.configuration.AckDeadline != "":
		ackDeadlineString = p.configuration.AckDeadline
	default:
		ackDeadlineString = "10s"
	}

	return time.ParseDuration(ackDeadlineString)
}

func (p *pubsub) setAndValidateGoogleApplicationCredentials() error {
	if p.configuration.NoCredentials {
		return nil
	}
	if p.configuration.Credentials.Contents != "" {

		// dump contents to a file and use
		serviceAccountFilePath := "/tmp/service-account.json"

		if err := ioutil.WriteFile(serviceAccountFilePath,
			[]byte(p.configuration.Credentials.Contents),
			0600); err != nil {
			return errors.Wrap(err, "Failed to write temporary service account")
		}
		if err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", serviceAccountFilePath); err != nil {
			return errors.Wrap(err, "Failed to set credentials env")
		}
	}

	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		return errors.New(
			"GOOGLE_APPLICATION_CREDENTIALS env must be filled with a valid service account file path")
	}
	return nil
}

func (p *pubsub) createOrUseSubscription(ctx context.Context,
	subscriptionID string,
	ackDeadline time.Duration,
	subscriptionConfig *Subscription) (*pubsubClient.Subscription, error) {
	var err error
	var created bool
	var subscription *pubsubClient.Subscription

	if !subscriptionConfig.SkipCreate {
		p.Logger.DebugWith("Creating subscription",
			"sid", subscriptionID,
			"ackDeadline", ackDeadline,
			"topic", subscriptionConfig.Topic)
		subscription, err = p.client.CreateSubscription(ctx, subscriptionID, pubsubClient.SubscriptionConfig{
			Topic:       p.client.Topic(subscriptionConfig.Topic),
			AckDeadline: ackDeadline,
		})
		if err != nil && !subscriptionConfig.Shared {
			return nil, errors.Wrap(err, "Failed to create subscription")
		}
		created = true
	}

	// use
	if subscription == nil {
		subscription = p.client.Subscription(subscriptionID)
	}

	// https://godoc.org/cloud.google.com/go/pubsub#ReceiveSettings
	// TODO: load all ReceiveSettings from subscriptionConfig
	subscription.ReceiveSettings.NumGoroutines = subscriptionConfig.MaxNumWorkers
	subscription.ReceiveSettings.Synchronous = subscriptionConfig.Synchronous

	p.Logger.DebugWith("Resolved subscription",
		"sid", subscriptionID,
		"topic", subscriptionConfig.Topic,
		"created", created,
		"ackDeadline", ackDeadline,
		"err", err)

	return subscription, nil

}
