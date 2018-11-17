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

package mqtt

import (
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	mqttclient "github.com/eclipse/paho.mqtt.golang"
	"github.com/nuclio/logger"
)

type mqtt struct {
	trigger.AbstractTrigger
	event         Event
	configuration *Configuration

	// TODO: allow configuring per-topic worker allocators to allow for things like:
	// - in-order handling of a topic messages (unique worker allocator with 1 worker for a topic)
	// - disallowing parallel handling of topics (e.g. topic1, topic2 share worker allocator so that only one handler
	//   is called at any given time
	perTopicWorkerAllocator map[string]worker.Allocator
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {

	newTrigger := mqtt{
		AbstractTrigger: trigger.AbstractTrigger{
			ID:              configuration.ID,
			Logger:          parentLogger.GetChild(configuration.ID),
			WorkerAllocator: workerAllocator,
			Class:           "async",
			Kind:            "mqtt",
		},
		configuration: configuration,
	}

	return &newTrigger, nil
}

func (m *mqtt) Start(checkpoint functionconfig.Checkpoint) error {
	m.Logger.InfoWith("Starting", "brokerUrl", m.configuration.URL)

	if err := m.createSubscriptions(); err != nil {
		return errors.Wrap(err, "Failed to create broker resources")
	}

	return nil
}

func (m *mqtt) Stop(force bool) (functionconfig.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (m *mqtt) GetConfig() map[string]interface{} {
	return common.StructureToMap(m.configuration)
}

func (m *mqtt) createSubscriptions() error {
	m.Logger.InfoWith("Creating subscriptions",
		"brokerUrl", m.configuration.URL,
		"subscriptions", m.configuration.Subscriptions,
		"clientID", m.configuration.ClientID)

	clientOptions := mqttclient.NewClientOptions()

	clientOptions.AddBroker(m.configuration.URL)
	clientOptions.SetProtocolVersion(uint(m.configuration.ProtocolVersion))

	if m.configuration.Username != "" {
		clientOptions.SetUsername(m.configuration.Username)
	}

	if m.configuration.Password != "" {
		clientOptions.SetPassword(m.configuration.Password)
	}

	clientOptions.SetClientID(m.configuration.ClientID)

	client := mqttclient.NewClient(clientOptions)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return errors.Wrap(token.Error(), "Failed to connect to broker")
	}

	// subscribe to topics
	if token := client.SubscribeMultiple(m.subscriptionsToFilters(m.configuration.Subscriptions),
		m.handleMessage); token.Wait() && token.Error() != nil {
		return errors.Wrap(token.Error(), "Failed to subscribe to topics")
	}

	return nil
}

func (m *mqtt) subscriptionsToFilters(subscriptions []Subscription) map[string]byte {
	filters := map[string]byte{}

	// add filter
	for _, subscription := range subscriptions {
		filters[subscription.Topic] = byte(subscription.QOS)
	}

	return filters
}

func (m *mqtt) handleMessage(client mqttclient.Client, message mqttclient.Message) {

	// get a worker for this message
	workerInstance, workerAllocator, err := m.allocateWorker(message)
	if err != nil {
		m.Logger.WarnWith("Failed to allocate worker, message dropped", "topic", message.Topic())
		return
	}

	m.SubmitEventToWorker(nil, workerInstance, &Event{message: message}) // nolint: errcheck

	workerAllocator.Release(workerInstance)
}

func (m *mqtt) allocateWorker(message mqttclient.Message) (*worker.Worker, worker.Allocator, error) {
	var workerAllocator worker.Allocator

	// if there's a per-topic worker allocator, first get worker allocator
	if m.perTopicWorkerAllocator != nil {

		// try to get the worker allocator
		workerAllocator = m.perTopicWorkerAllocator[message.Topic()]
	}

	// if there's no allocated worker allocator (either because per topic worker allocator is not enabled, or it
	// is but there's no specific worker allocator for this topic) - use the trigger's worker allocator
	if workerAllocator == nil {
		workerAllocator = m.WorkerAllocator
	}

	workerAvailabilityTimeout := time.Duration(m.configuration.WorkerAvailabilityTimeoutMilliseconds) * time.Millisecond

	// try to allocate the worker
	workerInstance, err := workerAllocator.Allocate(workerAvailabilityTimeout)

	return workerInstance, workerAllocator, err
}
