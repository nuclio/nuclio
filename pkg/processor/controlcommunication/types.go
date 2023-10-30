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

package controlcommunication

import (
	"bufio"
	"sync"

	"github.com/nuclio/errors"
)

type ControlMessageKind string

const (
	StreamMessageAckKind ControlMessageKind = "streamMessageAck"
)

// TODO: move to nuclio-sdk-go
type ControlMessage struct {
	Kind       ControlMessageKind
	Attributes map[string]interface{}
}

type ControlMessageAttributesExplicitAck struct {
	Topic     string `json:"topic"`
	Partition int32  `json:"partition"`
	Offset    int64  `json:"offset"`
}

type ControlConsumer struct {
	channels []chan *ControlMessage
	kind     ControlMessageKind
}

// NewControlConsumer creates a new control consumer
func NewControlConsumer(kind ControlMessageKind) *ControlConsumer {

	return &ControlConsumer{
		channels: make([]chan *ControlMessage, 0),
		kind:     kind,
	}
}

// GetKind returns the kind of the consumer
func (c *ControlConsumer) GetKind() ControlMessageKind {
	return c.kind
}

// Send broadcasts a message to all subscribed channels
func (c *ControlConsumer) Send(message *ControlMessage) error {

	wg := sync.WaitGroup{}
	wg.Add(len(c.channels))
	for _, channel := range c.channels {

		go func(channel chan *ControlMessage, message *ControlMessage) {
			channel <- message
			wg.Done()
		}(channel, message)
	}

	wg.Wait()
	return nil
}

func (c *ControlConsumer) addChannel(channel chan *ControlMessage) {
	c.channels = append(c.channels, channel)
}

func (c *ControlConsumer) deleteChannel(channelToDelete chan *ControlMessage) {
	// remove the channel from the consumer
	for i, channel := range c.channels {
		if channel == channelToDelete {
			c.channels = append(c.channels[:i], c.channels[i+1:]...)
			break
		}
	}
}

type ControlMessageBroker interface {

	// WriteControlMessage writes a control message to the control communication
	WriteControlMessage(message *ControlMessage) error

	// ReadControlMessage reads a control message from the control communication
	ReadControlMessage(reader *bufio.Reader) (*ControlMessage, error)

	// SendToConsumers sends a control message to all consumers
	SendToConsumers(message *ControlMessage) error

	// Subscribe subscribes channel to a control message kind
	Subscribe(kind ControlMessageKind, channel chan *ControlMessage) error

	// Unsubscribe unsubscribes channel from a control message kind
	Unsubscribe(kind ControlMessageKind, channel chan *ControlMessage) error
}

type AbstractControlMessageBroker struct {
	Consumers   []*ControlConsumer
	channelLock sync.Mutex
}

// NewAbstractControlMessageBroker creates a new abstract control message broker
func NewAbstractControlMessageBroker() *AbstractControlMessageBroker {
	return &AbstractControlMessageBroker{
		Consumers:   make([]*ControlConsumer, 0),
		channelLock: sync.Mutex{},
	}
}

func (acmb *AbstractControlMessageBroker) WriteControlMessage(message *ControlMessage) error {
	return nil
}

func (acmb *AbstractControlMessageBroker) ReadControlMessage(reader *bufio.Reader) (*ControlMessage, error) {
	return nil, nil
}

func (acmb *AbstractControlMessageBroker) SendToConsumers(message *ControlMessage) error {
	acmb.channelLock.Lock()
	defer acmb.channelLock.Unlock()

	for _, consumer := range acmb.Consumers {
		if consumer.GetKind() == message.Kind {
			if err := consumer.Send(message); err != nil {
				return errors.Wrap(err, "Failed to send message to consumer")
			}
		}
	}

	return nil
}

func (acmb *AbstractControlMessageBroker) Subscribe(kind ControlMessageKind, channel chan *ControlMessage) error {

	// acquire lock to prevent concurrent access to the consumers and channels
	acmb.channelLock.Lock()
	defer acmb.channelLock.Unlock()

	// create consumers if they don't exist
	if acmb.Consumers == nil {
		acmb.Consumers = make([]*ControlConsumer, 0)
	}

	// Add the consumer to the list of the relevant kind
	for _, consumer := range acmb.Consumers {
		if consumer.GetKind() == kind {
			consumer.addChannel(channel)
			return nil
		}
	}

	// consumer for the kind doesn't exist, create one
	consumer := NewControlConsumer(kind)
	consumer.addChannel(channel)
	acmb.Consumers = append(acmb.Consumers, consumer)

	return nil
}

func (acmb *AbstractControlMessageBroker) Unsubscribe(kind ControlMessageKind, channel chan *ControlMessage) error {

	// acquire lock to prevent concurrent access to the consumers and channels
	acmb.channelLock.Lock()
	defer acmb.channelLock.Unlock()

	// Find the consumer with relevant kind
	for _, consumer := range acmb.Consumers {
		if consumer.GetKind() == kind {
			consumer.deleteChannel(channel)
			return nil
		}
	}
	return nil
}
