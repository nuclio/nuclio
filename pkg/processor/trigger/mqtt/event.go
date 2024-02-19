/*
Copyright 2023 The Nuclio Authors.

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

package mqtt

import (
	mqttclient "github.com/eclipse/paho.mqtt.golang"
	"github.com/nuclio/nuclio-sdk-go"
	"strconv"
)

// Event allows access to the MQTT message
type Event struct {
	nuclio.AbstractEvent
	message mqttclient.Message
}

func (e *Event) GetBody() []byte {
	return e.message.Payload()
}

// GetURL returns the topic of the event
func (e *Event) GetURL() string {
	return e.message.Topic()
}

// GetTopic returns the topic of the event
func (e *Event) GetTopic() string {
	return e.message.Topic()
}

// GetID returns the message ID
func (e *Event) GetID() nuclio.ID {
	return nuclio.ID(strconv.Itoa(int(e.message.MessageID())))
}
