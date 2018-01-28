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
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/streadway/amqp"
)

// allows accessing an amqp.Delivery
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
