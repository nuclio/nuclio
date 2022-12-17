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

package nats

import (
	natsio "github.com/nats-io/nats.go"
	"github.com/nuclio/nuclio-sdk-go"
)

// Event allows accessing an amqp.Delivery
type Event struct {
	nuclio.AbstractEvent
	natsMessage *natsio.Msg
}

func (e *Event) GetBody() []byte {
	return e.natsMessage.Data
}

// GetHeaders loads all headers into a map of string / interface{}
func (e *Event) GetHeaders() map[string]interface{} {

	// convert headers to map[string]interface{}
	headers := map[string]interface{}{}
	for key, value := range e.natsMessage.Header {
		headers[key] = value
	}
	return headers
}

func (e *Event) GetPath() string {
	return e.natsMessage.Subject
}

func (e *Event) GetSize() int {
	return len(e.natsMessage.Data)
}

func (e *Event) NATSMessage() *natsio.Msg {
	return e.natsMessage
}
