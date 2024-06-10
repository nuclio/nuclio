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
	"bytes"
	"encoding/binary"
	"io"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/vmihailenco/msgpack/v4"
)

// EventMsgPackEncoder encodes nuclio events as MsgPack
type EventMsgPackEncoder struct {
	logger  logger.Logger
	writer  io.Writer
	buf     bytes.Buffer
	encoder *msgpack.Encoder
}

// NewEventMsgPackEncoder returns a new MsgPackEncoder
func NewEventMsgPackEncoder(logger logger.Logger, writer io.Writer) *EventMsgPackEncoder {
	eventMsgPackEncoder := EventMsgPackEncoder{logger: logger, writer: writer}
	eventMsgPackEncoder.encoder = msgpack.NewEncoder(&eventMsgPackEncoder.buf)
	return &eventMsgPackEncoder
}

// Encode writes the JSON encoding of event to the stream, followed by a newline character
func (e *EventMsgPackEncoder) Encode(object interface{}) error {
	prepareOneEvent := func(event nuclio.Event) map[string]interface{} {
		eventToEncode := eventAsMap(event)

		// if the body is map[string]interface{} we probably got a cloud event with a structured data member
		if bodyObject, isMapStringInterface := event.GetBodyObject().(map[string]interface{}); isMapStringInterface {
			eventToEncode["body"] = bodyObject
		} else {
			eventToEncode["body"] = event.GetBody()
		}
		return eventToEncode
	}
	var preparedEvent interface{}
	switch typedEvent := object.(type) {
	case nuclio.Event:
		preparedEvent = prepareOneEvent(typedEvent)
	case []nuclio.Event:
		eventSlice := make([]map[string]interface{}, 0)
		for _, event := range typedEvent {
			eventSlice = append(eventSlice, prepareOneEvent(event))
		}
		preparedEvent = eventSlice
	}

	e.buf.Reset()
	if err := e.encoder.Encode(preparedEvent); err != nil {
		return errors.Wrap(err, "Failed to encode message")
	}

	// write the encoded size message to the socket
	if err := binary.Write(e.writer, binary.BigEndian, int32(e.buf.Len())); err != nil {
		return errors.Wrap(err, "Failed to write message size to socket")
	}

	// write the encoded message content to the socket
	bs := e.buf.Bytes()
	if _, err := e.writer.Write(bs); err != nil {
		return errors.Wrap(err, "Failed to write message to socket")
	}

	return nil
}
