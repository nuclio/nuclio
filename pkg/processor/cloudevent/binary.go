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
	"time"

	"github.com/nuclio/nuclio-sdk-go"
)

// Binary wraps a nuclio.Event with a cloudevent whose data is encoded in the nuclio.Event body.
type Binary struct {
	wrappedEvent
}

// SetEvent wraps a nuclio event
func (s *Binary) SetEvent(event nuclio.Event) error {
	s.event = event

	// set trigger info provider to ourselves
	s.event.SetTriggerInfoProvider(s)

	return nil
}

// GetID returns the ID of the event
func (s *Binary) GetID() nuclio.ID {
	return nuclio.ID(s.event.GetHeaderString("CE-EventID"))
}

// get the class of source (sync, async, etc)
func (s *Binary) GetClass() string {
	return "unsupported"
}

// get specific kind of source (http, rabbit mq, etc)
func (s *Binary) GetKind() string {
	return s.event.GetHeaderString("CE-Source")
}

// GetTimestamp returns when the event originated
func (s *Binary) GetTimestamp() time.Time {
	parsedTime, err := time.Parse(time.RFC3339, s.event.GetHeaderString("CE-EventTime"))
	if err != nil {
		return time.Time{}
	}

	return parsedTime
}

// GetType returns the type of event
func (s *Binary) GetType() string {
	return s.event.GetHeaderString("CE-EventType")
}

// GetTypeVersion returns the version of the type
func (s *Binary) GetTypeVersion() string {
	return s.event.GetHeaderString("CE-EventTypeVersion")
}

// GetVersion returns the version of the event
func (s *Binary) GetVersion() string {
	return s.event.GetHeaderString("CE-CloudEventsVersion")
}
