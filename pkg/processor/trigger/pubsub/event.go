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

package pubsub

import (
	pubsubClient "cloud.google.com/go/pubsub"
	"github.com/nuclio/nuclio-sdk-go"
)

// Event stores a whole pubsub message
type Event struct {
	nuclio.AbstractEvent
	message *pubsubClient.Message
	topic   string
}

// GetBody returns a message data
func (e *Event) GetBody() []byte {
	return e.message.Data
}

// GetSize returns data length
func (e *Event) GetSize() int {
	return len(e.message.Data)
}

// GetURL returns the URL of the event
func (e *Event) GetURL() string {
	return e.topic
}

// GetPath returns the path of the event
func (e *Event) GetPath() string {
	return e.topic
}

// GetHeaderByteSlice returns the header by name as a byte slice
func (e *Event) GetHeaderByteSlice(key string) []byte {
	return []byte(e.message.Attributes[key])
}

// GetHeader returns the header by name as an interface{}
func (e *Event) GetHeader(key string) interface{} {
	return e.message.Attributes[key]
}

// GetHeaders loads all headers into a map of string / interface{}
func (e *Event) GetHeaders() map[string]interface{} {
	headers := make(map[string]interface{}, len(e.message.Attributes))
	for headerKey, headerValue := range e.message.Attributes {
		headers[headerKey] = headerValue
	}
	return headers
}

// GetHeaderString returns the header by name as a string
func (e *Event) GetHeaderString(key string) string {
	return e.message.Attributes[key]
}
