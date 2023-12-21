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

package v3iostream

import (
	"time"

	"github.com/nuclio/nuclio-sdk-go"
	"github.com/v3io/v3io-go/pkg/dataplane"
)

type Event struct {
	nuclio.AbstractEvent
	record *v3io.StreamRecord
}

func (e *Event) GetBody() []byte {
	return e.record.Data
}

func (e *Event) GetSize() int {
	return len(e.GetBody())
}

func (e *Event) GetShardID() int {
	return *e.record.ShardID
}

func (e *Event) GetOffset() int {
	return int(e.record.SequenceNumber)
}

func (e *Event) GetTimestamp() time.Time {
	return time.Unix(int64(e.record.ArrivalTimeSec), 0)
}
