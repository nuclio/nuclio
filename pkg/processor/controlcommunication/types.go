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
	DrainDoneMessageKind ControlMessageKind = "drainDone"
)

func GetAllControlMessageKinds() []ControlMessageKind {
	return []ControlMessageKind{StreamMessageAckKind, DrainDoneMessageKind}
}

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
