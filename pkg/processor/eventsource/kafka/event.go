// +build kafka

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

package kafka

import (
	"github.com/nuclio/nuclio-sdk"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type Event struct {
	nuclio.AbstractSync
	kafkaEvent kafka.Event
}

func (e *Event) GetBody() []byte {
	return []byte(e.kafkaEvent.String())
}

func (e *Event) GetSize() int {
	return len(e.kafkaEvent.String())
}
