package python

import (
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/nuclio/nuclio-sdk"
)

// EventJSONEncoder encodes nuclio events as JSON
type EventJSONEncoder struct {
	logger nuclio.Logger
	writer io.Writer
}

// NewEventJSONEncoder returns a new JSONEncoder
func NewEventJSONEncoder(logger nuclio.Logger, writer io.Writer) *EventJSONEncoder {
	return &EventJSONEncoder{logger, writer}
}

// Encode writes the JSON encoding of event to the stream, followed by a newline character.
func (je *EventJSONEncoder) Encode(event nuclio.Event) error {
	je.logger.DebugWith("encoding", "event", event)
	src := event.GetSource()
	body := base64.StdEncoding.EncodeToString(event.GetBody())
	obj := map[string]interface{}{
		"version": event.GetVersion(),
		"id":      event.GetID().String(),
		"source": map[string]string{
			"class": src.GetClass(),
			"kind":  src.GetKind(),
		},
		"content-type": event.GetContentType(),
		"body":         body,
		"size":         event.GetSize(),
		"headers":      event.GetHeaders(),
		"timestamp":    event.GetTimestamp(),
		"path":         event.GetPath(),
		"url":          event.GetURL(),
	}

	return json.NewEncoder(je.writer).Encode(obj)
}
