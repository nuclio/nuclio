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
	"fmt"
	"github.com/nuclio/errors"
	"sync"
)

type ControlConsumer struct {
	channels []chan *ControlMessage
	kind     ControlMessageKind
	lock     sync.Mutex
}

// NewControlConsumer creates a new control consumer
func NewControlConsumer(kind ControlMessageKind) *ControlConsumer {

	return &ControlConsumer{
		channels: make([]chan *ControlMessage, 0),
		kind:     kind,
		lock:     sync.Mutex{},
	}
}

// GetKind returns the kind of the consumer
func (c *ControlConsumer) GetKind() ControlMessageKind {
	return c.kind
}

// BroadcastAndCloseSubscriptions sends a message to all subscribed channels and deletes all subscriptions after
func (c *ControlConsumer) BroadcastAndCloseSubscriptions(message *ControlMessage) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// do nothing if channels is an empty slice
	if len(c.channels) == 0 {
		return nil
	}
	// write message to the all channels
	for _, channel := range c.channels {
		go func(channel chan *ControlMessage, message *ControlMessage) {
			defer func() {
				// if the channel is closed before the message is read, it will result in a panic
				if err := recover(); err != nil {
					fmt.Println("Recovered in BroadcastAndCloseSubscriptions", err)
				}
			}()
			channel <- message
		}(channel, message)
	}

	// delete the channel from subscription
	c.channels = make([]chan *ControlMessage, 0)

	return nil
}

// Broadcast broadcasts a message to all subscribed channels
func (c *ControlConsumer) Broadcast(message *ControlMessage) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	wg := sync.WaitGroup{}
	wg.Add(len(c.channels))
	for _, channel := range c.channels {

		go func(channel chan *ControlMessage, message *ControlMessage) {
			defer func() {
				// if the channel is closed before the message is read, it will result in a panic
				if err := recover(); err != nil {
					fmt.Println("Recovered in BroadcastAndCloseSubscriptions", err)
				}
			}()
			defer wg.Done()
			channel <- message
		}(channel, message)
	}

	wg.Wait()
	return nil
}

func (c *ControlConsumer) addChannel(channel chan *ControlMessage) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.channels = append(c.channels, channel)
}

func (c *ControlConsumer) deleteChannel(channelToDelete chan *ControlMessage) {
	c.lock.Lock()
	defer c.lock.Unlock()

	// remove the channel from the consumer
	for i, channel := range c.channels {
		if channel == channelToDelete {
			c.channels = append(c.channels[:i], c.channels[i+1:]...)
			break
		}
	}
}

type AbstractControlMessageBroker struct {
	Consumers []*ControlConsumer
}

// NewAbstractControlMessageBroker creates a new abstract control message broker
func NewAbstractControlMessageBroker() *AbstractControlMessageBroker {
	// create a consumer for each control message kind
	controlMessageKinds := getAllControlMessageKinds()
	consumers := make([]*ControlConsumer, len(controlMessageKinds))
	for index, kind := range controlMessageKinds {
		consumers[index] = NewControlConsumer(kind)
	}
	return &AbstractControlMessageBroker{
		Consumers: consumers,
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
			switch message.Kind {
			case DrainDoneMessageKind:
				if err := consumer.BroadcastAndCloseSubscriptions(message); err != nil {
					return errors.Wrap(err, fmt.Sprintf("Failed to send message of kind `%s` to consumer",
						message.Kind))
				}
			case StreamMessageAckKind:
				if err := consumer.Broadcast(message); err != nil {
					return errors.Wrap(err, fmt.Sprintf("Failed to broadcast message of kind `%s` to consumer",
						message.Kind))
				}
			default:
				return errors.New(fmt.Sprintf("Received unknown control message of `%s` kind", message.Kind))
			}
		}
	}

	return nil
}

func (acmb *AbstractControlMessageBroker) Subscribe(kind ControlMessageKind, channel chan *ControlMessage) error {
	if consumer, err := acmb.getConsumer(kind); err != nil {
		return err
	} else {
		consumer.addChannel(channel)
	}
	return nil
}

func (acmb *AbstractControlMessageBroker) Unsubscribe(kind ControlMessageKind, channel chan *ControlMessage) error {
	if consumer, err := acmb.getConsumer(kind); err != nil {
		return err
	} else {
		consumer.deleteChannel(channel)
	}
	return nil
}

func (acmb *AbstractControlMessageBroker) getConsumer(kind ControlMessageKind) (*ControlConsumer, error) {
	for _, consumer := range acmb.Consumers {
		if consumer.GetKind() == kind {
			return consumer, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("Consumer for message kind `%s` does not exist", kind))
}
