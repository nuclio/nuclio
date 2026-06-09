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
)

type ControlMessageKind string

const (
	StreamMessageAckKind ControlMessageKind = "streamMessageAck"
	LogMessageKind       ControlMessageKind = "log"
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

// Subscription is a handle to a control-message subscription.
//
// The broker owns the underlying channel: it is the sole writer and the sole closer.
// Callers receive messages by ranging or selecting on C() and must invoke Close()
// when they are no longer interested in messages. After Close() returns, the channel
// is closed and no further sends to it will occur, so it is safe for the broker to
// drop late messages without panicking on a closed-channel write.
//
// Close is idempotent and safe to call from multiple goroutines.
type Subscription interface {

	// C returns the receive-only channel for control messages of the subscribed kind.
	// The channel is closed by the broker exactly once, after Close() returns.
	C() <-chan *ControlMessage

	// Close unsubscribes from the broker and closes the channel. Safe to call
	// multiple times.
	Close()
}

// ControlMessageBroker routes control messages received from a runtime wrapper to
// subscribers interested in specific message kinds.
type ControlMessageBroker interface {

	// WriteControlMessage writes a control message to the control communication
	WriteControlMessage(message *ControlMessage) error

	// ReadControlMessage reads a control message from the control communication
	ReadControlMessage(reader *bufio.Reader) (*ControlMessage, error)

	// SendToConsumers fans out a control message to all subscriptions whose kind matches.
	// The call returns once delivery has been scheduled; per-subscription delivery
	// happens asynchronously and is bounded by the subscription's lifetime — a
	// subscription that is Close()d while a send is in flight drops the message
	// instead of blocking the broker.
	SendToConsumers(message *ControlMessage) error

	// Subscribe creates a new subscription for the given control message kind.
	// The returned Subscription owns its channel; the caller must invoke
	// Subscription.Close() to release resources.
	Subscribe(kind ControlMessageKind) (Subscription, error)
}

