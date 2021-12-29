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
func (e *EventMsgPackEncoder) Encode(event nuclio.Event) error {
	eventToEncode := eventAsMap(event)

	// if the body is map[string]interface{} we probably got a cloud event with a structured data member
	if bodyObject, isMapStringInterface := event.GetBodyObject().(map[string]interface{}); isMapStringInterface {
		eventToEncode["body"] = bodyObject
	} else {
		eventToEncode["body"] = event.GetBody()
	}

	e.buf.Reset()
	if err := e.encoder.Encode(eventToEncode); err != nil {
		return errors.Wrap(err, "Failed to encode message")
	}

	if err := binary.Write(e.writer, binary.BigEndian, int32(e.buf.Len())); err != nil {
		return errors.Wrap(err, "Failed to write message size to socket")
	}

	bs := e.buf.Bytes()
	if _, err := e.writer.Write(bs); err != nil {
		return errors.Wrap(err, "Failed to write message to socket")
	}

	return nil
}
