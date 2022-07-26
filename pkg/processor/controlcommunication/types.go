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

package controlcommunication

import (
	"bufio"

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

type ControlConsumer struct {
	Channels []chan *ControlMessage
	kind     ControlMessageKind
}

func NewControlConsumer(kind ControlMessageKind) *ControlConsumer {

	return &ControlConsumer{
		Channels: make([]chan *ControlMessage, 0),
		kind:     kind,
	}
}

// GetKind returns the kind of the consumer
func (c *ControlConsumer) GetKind() ControlMessageKind {
	return c.kind
}

// Send broadcasts a message to all subscribed channels
func (c *ControlConsumer) Send(message *ControlMessage) error {

	for _, channel := range c.Channels {
		channel <- message
	}

	return nil
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
}

type AbstractControlMessageBroker struct {
	Consumers []*ControlConsumer
}

func NewAbstractControlMessageBroker() *AbstractControlMessageBroker {
	return &AbstractControlMessageBroker{
		Consumers: make([]*ControlConsumer, 0),
	}
}

func (acmb *AbstractControlMessageBroker) WriteControlMessage(message *ControlMessage) error {
	return nil
}

func (acmb *AbstractControlMessageBroker) ReadControlMessage(reader *bufio.Reader) (*ControlMessage, error) {
	return nil, nil
}

func (acmb *AbstractControlMessageBroker) SendToConsumers(message *ControlMessage) error {
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

	// create consumers if they don't exist
	if acmb.Consumers == nil {
		acmb.Consumers = make([]*ControlConsumer, 0)
	}

	// Add the consumer to the list of the relevant kind
	for _, consumer := range acmb.Consumers {
		if consumer.GetKind() == kind {
			consumer.Channels = append(consumer.Channels, channel)
			return nil
		}
	}

	// consumer for the kind doesn't exist, create one
	consumer := NewControlConsumer(kind)
	consumer.Channels = append(consumer.Channels, channel)
	acmb.Consumers = append(acmb.Consumers, consumer)

	return nil
}
