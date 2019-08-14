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

package rpc

import (
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

// EventJSONEncoder encodes nuclio events as JSON
type EventJSONEncoder struct {
	logger logger.Logger
	writer io.Writer
}

// NewEventJSONEncoder returns a new JSONEncoder
func NewEventJSONEncoder(logger logger.Logger, writer io.Writer) *EventJSONEncoder {
	return &EventJSONEncoder{logger, writer}
}

// Encode writes the JSON encoding of event to the stream, followed by a newline character
func (e *EventJSONEncoder) Encode(event nuclio.Event) error {
	e.logger.DebugWith("Sending event to wrapper", "size", len(event.GetBody()))

	eventToEncode := eventAsMap(event)

	// if the body is map[string]interface{} we probably got a cloud event with a structured data member
	if bodyObject, isMapStringInterface := event.GetBodyObject().(map[string]interface{}); isMapStringInterface {
		eventToEncode["body"] = bodyObject
	} else {
		// otherwise, just encode body to base64
		eventToEncode["body"] = base64.StdEncoding.EncodeToString(event.GetBody())
	}

	return json.NewEncoder(e.writer).Encode(eventToEncode)
}
