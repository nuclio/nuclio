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

package rpc

import (
	"bufio"
	"encoding/json"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type rpcControlMessageBroker struct {
	*controlcommunication.AbstractControlMessageBroker
	ControlMessageEventEncoder EventEncoder
	logger                     logger.Logger
}

// NewRpcControlMessageBroker creates a new RPC control message broker
func NewRpcControlMessageBroker(encoder EventEncoder, logger logger.Logger, abstractControlMessageBroker *controlcommunication.AbstractControlMessageBroker) *rpcControlMessageBroker {

	if abstractControlMessageBroker == nil {
		abstractControlMessageBroker = controlcommunication.NewAbstractControlMessageBroker()
	}

	return &rpcControlMessageBroker{
		AbstractControlMessageBroker: abstractControlMessageBroker,
		ControlMessageEventEncoder:   encoder,
		logger:                       logger.GetChild("controlMessageBroker"),
	}
}

// WriteControlMessage writes control message to the control socket using MSGPack encoding
func (b *rpcControlMessageBroker) WriteControlMessage(message *controlcommunication.ControlMessage) error {

	// send control message as a nuclio event, this will be handled by the wrapper
	controlMessageEvent := controlcommunication.NewControlMessageEvent(message)

	if err := b.ControlMessageEventEncoder.Encode(controlMessageEvent); err != nil {
		return errors.Wrapf(err, "Can't encode control message event: %+v", controlMessageEvent)
	}

	return nil
}

// ReadControlMessage reads from the control socket and unpacks it into a control message
func (b *rpcControlMessageBroker) ReadControlMessage(reader *bufio.Reader) (*controlcommunication.ControlMessage, error) {

	// TODO: use msgpack to decode the message

	// read data from reader
	data, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, errors.Wrap(err, string(common.FailedReadFromEventConnection))
	}

	unmarshalledControlMessage := &controlcommunication.ControlMessage{}

	// try to unmarshall the data
	if err := json.Unmarshal(data, unmarshalledControlMessage); err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal control message")
	}

	return unmarshalledControlMessage, nil
}
