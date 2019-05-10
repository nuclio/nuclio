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
	nuclio "github.com/nuclio/nuclio-sdk-go"
)

type AbstractTrigger struct {
	trigger.AbstractTrigger
	event         Event
	configuration *Configuration
	MQTTClient    mqttclient.Client

	// TODO: allow configuring per-topic worker allocators to allow for things like:
	// - in-order handling of a topic messages (unique worker allocator with 1 worker for a topic)
	// - disallowing parallel handling of topics (e.g. topic1, topic2 share worker allocator so that only one handler
	//   is called at any given time
	perTopicWorkerAllocator map[string]worker.Allocator
}

func NewAbstractTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (*AbstractTrigger, error) {
	instanceLogger := parentLogger.GetChild(configuration.ID)

	abstractTrigger, err := trigger.NewAbstractTrigger(instanceLogger,
		workerAllocator,
		&configuration.Configuration,
		"async",
		"mqtt")
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger := AbstractTrigger{
		AbstractTrigger: abstractTrigger,
		configuration:   configuration,
	}

	return &newTrigger, nil
}

func (t *AbstractTrigger) Start(checkpoint functionconfig.Checkpoint) error {
	return t.Connect()
}

func (t *AbstractTrigger) Stop(force bool) (functionconfig.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (t *AbstractTrigger) GetConfig() map[string]interface{} {
	return common.StructureToMap(t.configuration)
}

func (t *AbstractTrigger) Connect() error {
	t.Logger.InfoWith("Connecting")

	clientOptions, err := t.createClientOptions()
	if err != nil {
		return errors.Wrap(err, "Failed to create client options")
	}

	t.MQTTClient, err = t.createClient(clientOptions)
	if err != nil {
		return errors.Wrap(err, "Failed to create client")
	}

	if err := t.createSubscriptions(clientOptions); err != nil {
		return errors.Wrap(err, "Failed to create broker resources")
	}

	return nil
}

func (t *AbstractTrigger) createClient(clientOptions *mqttclient.ClientOptions) (mqttclient.Client, error) {
	t.Logger.InfoWith("Creating client",
		"brokerUrl", t.configuration.URL,
		"clientID", t.configuration.ClientID,
		"username", t.configuration.Username,
		"protocolVersion", t.configuration.ProtocolVersion)

	client := mqttclient.NewClient(clientOptions)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, errors.Wrap(token.Error(), "Failed to connect to broker")
	}

	return client, nil
}

func (t *AbstractTrigger) createClientOptions() (*mqttclient.ClientOptions, error) {
	clientOptions := mqttclient.NewClientOptions()

	clientOptions.AddBroker(t.configuration.URL)
	clientOptions.SetProtocolVersion(uint(t.configuration.ProtocolVersion))

	if t.configuration.Username != "" {
		clientOptions.SetUsername(t.configuration.Username)
	}

	if t.configuration.Password != "" {
		clientOptions.SetPassword(t.configuration.Password)
	}

	clientOptions.SetClientID(t.configuration.ClientID)

	return clientOptions, nil
}

func (t *AbstractTrigger) createSubscriptions(clientOptions *mqttclient.ClientOptions) error {
	t.Logger.InfoWith("Creating subscriptions",
		"subscriptions", t.configuration.Subscriptions)

	// subscribe to topics
	if token := t.MQTTClient.SubscribeMultiple(t.subscriptionsToFilters(t.configuration.Subscriptions),
		t.handleMessage); token.Wait() && token.Error() != nil {
		return errors.Wrap(token.Error(), "Failed to subscribe to topics")
	}

	return nil
}

func (t *AbstractTrigger) subscriptionsToFilters(subscriptions []Subscription) map[string]byte {
	filters := map[string]byte{}

	// add filter
	for _, subscription := range subscriptions {
		filters[subscription.Topic] = byte(subscription.QOS)
	}

	return filters
}

func (t *AbstractTrigger) handleMessage(client mqttclient.Client, message mqttclient.Message) {

	// get a worker for this message
	workerInstance, workerAllocator, err := t.allocateWorker(message)
	if err != nil {
		t.Logger.WarnWith("Failed to allocate worker, message dropped", "topic", message.Topic())
		return
	}

	response, processError := t.SubmitEventToWorker(nil, workerInstance, &Event{message: message})

	workerAllocator.Release(workerInstance)

	// no standard way to notify something went wrong through MQTT
	if processError != nil {
		return
	}

	// We may have a "response" message, verify it and publish it as a MQTT message
	switch typedResponse := response.(type) {
	case nuclio.Response:
		// Check status code
		if typedResponse.StatusCode != 200 {
			return
		}
		// We need to find at least a topic header
		mqttTopic := ""
		if mqttTopicValue, ok := typedResponse.Headers["MqttTopic"].(string); ok {
			mqttTopic = mqttTopicValue
		} else {
			return
		}
		// Optional headers
		mqttQos := byte(0)
		if mqttQosValue, ok := typedResponse.Headers["MqttQos"].(float64); ok &&
			mqttQosValue >= 0 &&
			mqttQosValue <= 2 {
			mqttQos = byte(mqttQosValue)
		}
		mqttRetain := false
		if mqttRetainValue, ok := typedResponse.Headers["MqttRetain"].(bool); ok {
			mqttRetain = mqttRetainValue
		}
		// Publish the message
		client.Publish(mqttTopic, mqttQos, mqttRetain, typedResponse.Body)
	}
}

func (t *AbstractTrigger) allocateWorker(message mqttclient.Message) (*worker.Worker, worker.Allocator, error) {
	var workerAllocator worker.Allocator

	// if there's a per-topic worker allocator, first get worker allocator
	if t.perTopicWorkerAllocator != nil {

		// try to get the worker allocator
		workerAllocator = t.perTopicWorkerAllocator[message.Topic()]
	}

	// if there's no allocated worker allocator (either because per topic worker allocator is not enabled, or it
	// is but there's no specific worker allocator for this topic) - use the trigger's worker allocator
	if workerAllocator == nil {
		workerAllocator = t.WorkerAllocator
	}

	workerAvailabilityTimeout := time.Duration(t.configuration.WorkerAvailabilityTimeoutMilliseconds) * time.Millisecond

	// try to allocate the worker
	workerInstance, err := workerAllocator.Allocate(workerAvailabilityTimeout)

	return workerInstance, workerAllocator, err
}
