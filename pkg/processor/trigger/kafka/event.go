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

package kafka

import (
	"time"

	"github.com/Shopify/sarama"
	"github.com/nuclio/nuclio-sdk-go"
)

type Event struct {
	nuclio.AbstractEvent
	kafkaMessage *sarama.ConsumerMessage
}

func (e *Event) GetBody() []byte {
	return e.kafkaMessage.Value
}

func (e *Event) GetSize() int {
	return len(e.kafkaMessage.Value)
}

func (e *Event) GetShardID() int {
	return int(e.kafkaMessage.Partition)
}

func (e *Event) GetPath() string {
	return e.kafkaMessage.Topic
}

func (e *Event) GetTimestamp() time.Time {
	return e.kafkaMessage.Timestamp
}

func (e *Event) GetHeaders() map[string]interface{} {
	return e.getHeadersAsMap()
}

// GetHeader returns the header by name as an interface{}
func (e *Event) GetHeader(key string) interface{} {
	return e.getHeadersAsMap()[key]
}

func (e *Event) getHeadersAsMap() map[string]interface{} {
	headersMap := map[string]interface{}{}

	if e.kafkaMessage == nil {
		return headersMap
	}

	if e.kafkaMessage.Headers == nil {
		return headersMap
	}

	// iterate over the header records and add each key-value to the map
	for _, headerRecord := range e.kafkaMessage.Headers {
		headersMap[string(headerRecord.Key)] = headerRecord.Value
	}

	return headersMap
}

func (e *Event) GetOffset() int {
	return int(e.kafkaMessage.Offset)
}
