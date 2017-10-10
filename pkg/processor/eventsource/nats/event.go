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
	"github.com/nuclio/nuclio/pkg/processor"

	natsio "github.com/nats-io/go-nats"
)

// allows accessing an amqp.Delivery
type Event struct {
	processor.AbstractSync
	natsMessage *natsio.Msg
}

func (e *Event) GetBody() []byte {
	return e.natsMessage.Data
}

func (e *Event) GetSize() int {
	return len(e.natsMessage.Data)
}

func (e *Event) NATSMessage() *natsio.Msg {
	return e.natsMessage
}
