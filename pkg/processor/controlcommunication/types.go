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

package controlcommunication

type ControlMessage struct {
	Kind       string
	Attributes map[string]interface{}
}

type ControlConsumer struct {
	Channels []chan *ControlMessage
	kind     string
}

// GetKind returns the kind of the consumer
func (c *ControlConsumer) GetKind() string {
	return c.kind
}

// Send sends the message to all channels in the consumer
func (c *ControlConsumer) Send(message *ControlMessage) error {

	for _, channel := range c.Channels {
		channel <- message
	}

	return nil
}

func NewControlConsumer(kind string) *ControlConsumer {

	return &ControlConsumer{
		Channels: make([]chan *ControlMessage, 0),
		kind:     kind,
	}
}
