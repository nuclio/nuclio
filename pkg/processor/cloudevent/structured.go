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

package cloudevent

import (
	"encoding/json"
	"time"

	"github.com/nuclio/nuclio-sdk-go"
)

// Structured wraps a nuclio.Event with a cloudevent whose data is encoded in the nuclio.Event body.
type Structured struct {
	wrappedEvent
	cloudEvent cloudEvent
}

// SetEvent wraps a nuclio event
func (s *Structured) SetEvent(event nuclio.Event) error {
	s.event = event

	// set trigger info provider to ourselves
	s.event.SetTriggerInfoProvider(s)

	body := string(event.GetBody())

	// parse the event body into the cloud event
	return json.Unmarshal([]byte(body), &s.cloudEvent)
}

// GetID returns the ID of the event
func (s *Structured) GetID() nuclio.ID {
	return nuclio.ID(s.cloudEvent.EventID)
}

// GetClass returns the class of source (sync, async, etc)
func (s *Structured) GetClass() string {
	return "unsupported"
}

// GetKind returns specific kind of source (http, rabbit mq, etc)
func (s *Structured) GetKind() string {
	return s.cloudEvent.Source
}

// GetTimestamp returns when the event originated
func (s *Structured) GetTimestamp() time.Time {
	return s.cloudEvent.EventTime
}

// GetContentType returns the content type of the body
func (s *Structured) GetContentType() string {
	return s.cloudEvent.ContentType
}

// GetBody returns the body of the event
func (s *Structured) GetBody() []byte {
	switch typedBody := s.cloudEvent.Data.(type) {
	case string:
		return []byte(typedBody)
	case []byte:
		return typedBody
	}

	return nil
}

// GetBodyObject returns the body of the event
func (s *Structured) GetBodyObject() interface{} {
	return s.cloudEvent.Data
}

// GetHeaders loads all headers into a map of string / interface{}
func (s *Structured) GetHeaders() map[string]interface{} {
	return s.cloudEvent.Extensions
}

// GetType returns the type of event
func (s *Structured) GetType() string {
	return s.cloudEvent.EventType
}

// GetTypeVersion returns the version of the type
func (s *Structured) GetTypeVersion() string {
	return s.cloudEvent.EventTypeVersion
}

// GetVersion returns the version of the event
func (s *Structured) GetVersion() string {
	return s.cloudEvent.CloudEventsVersion
}
