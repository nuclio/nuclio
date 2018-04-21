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
)

type cloudEvent struct {
	EventType          string                 `json:"eventType,omitempty"`
	EventTypeVersion   string                 `json:"eventTypeVersion,omitempty"`
	CloudEventsVersion string                 `json:"cloudEventsVersion,omitempty"`
	Source             string                 `json:"source,omitempty"`
	EventID            string                 `json:"eventID,omitempty"`
	EventTime          time.Time              `json:"eventTime,omitempty"`
	ContentType        string                 `json:"contentType,omitempty"`
	Extensions         map[string]interface{} `json:"extensions,omitempty"`
	Data               interface{}            `json:"data,omitempty"`
}
