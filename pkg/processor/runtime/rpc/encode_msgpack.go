package rpc

import (
	"bytes"
	"encoding/binary"
	"github.com/vmihailenco/msgpack"
	"io"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
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
	result := EventMsgPackEncoder{logger: logger, writer: writer}
	result.encoder = msgpack.NewEncoder(&result.buf)
	return &result
}

// Encode writes the JSON encoding of event to the stream, followed by a newline character
func (je *EventMsgPackEncoder) Encode(event nuclio.Event) error {
	je.logger.DebugWith("Sending event to wrapper", "size", len(event.GetBody()))

	eventToEncode := eventAsMap(event)

	// if the body is map[string]interface{} we probably got a cloud event with a structured data member
	if bodyObject, isMapStringInterface := event.GetBodyObject().(map[string]interface{}); isMapStringInterface {
		eventToEncode["body"] = bodyObject
	} else {
		eventToEncode["body"] = event.GetBody()
	}

	je.buf.Reset()
	err := je.encoder.Encode(eventToEncode)
	if err != nil {
		return err
	}

	err = binary.Write(je.writer, binary.BigEndian, int32(je.buf.Len()))
	if err != nil {
		return err
	}

	bs := je.buf.Bytes()
	_, err = je.writer.Write(bs)
	return err
}
