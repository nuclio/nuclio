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

package rabbitmq

import (
	"time"

	"github.com/nuclio/nuclio-sdk-go"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Event allows accessing an amqp.Delivery
type Event struct {
	nuclio.AbstractEvent
	message *amqp.Delivery
}

func (e *Event) GetContentType() string {
	return e.message.ContentType
}

func (e *Event) GetBody() []byte {
	return e.message.Body
}

func (e *Event) GetHeaders() map[string]interface{} {
	return e.message.Headers
}

func (e *Event) GetHeaderString(key string) string {

	value, found := e.message.Headers[key]
	if !found {
		return ""
	}

	switch typedValue := value.(type) {
	case string:
		return typedValue
	case []byte:
		return string(typedValue)
	default:
		return ""
	}
}

func (e *Event) GetID() nuclio.ID {
	return nuclio.ID(e.message.MessageId)
}

func (e *Event) GetMethod() string {
	return e.message.Type
}

func (e *Event) GetPath() string {
	return e.message.Exchange + "/" + e.message.RoutingKey
}

func (e *Event) GetTimestamp() time.Time {
	return e.message.Timestamp
}

func (e *Event) GetURL() string {
	return e.message.ReplyTo
}

func (e *Event) GetHeaderByteSlice(key string) []byte {
	value, found := e.message.Headers[key]
	if !found {
		return nil
	}

	switch typedValue := value.(type) {
	case string:
		return []byte(typedValue)
	case []byte:
		return typedValue
	default:
		return nil
	}
}
