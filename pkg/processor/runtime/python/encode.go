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

// Encode writes the JSON encoding of event to the stream, followed by a newline character
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
		"timestamp":    event.GetTimestamp().UTC().Unix(),
		"path":         event.GetPath(),
		"url":          event.GetURL(),
		"method":       event.GetMethod(),
	}

	return json.NewEncoder(je.writer).Encode(obj)
}
