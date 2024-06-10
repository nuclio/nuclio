/*
Copyright 2024 The Nuclio Authors.

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

package trigger

import (
	"time"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/google/uuid"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

type Batcher struct {
	Logger logger.Logger

	currentBatch chan *BatchedEventWithResponse
	batchIsFull  chan bool
}

type BatchedEventWithResponse struct {
	event        nuclio.Event
	responseChan *common.ChannelWithRecover
}

func NewBatcher(logger logger.Logger, batchSize int) *Batcher {
	return &Batcher{
		Logger:       logger,
		currentBatch: make(chan *BatchedEventWithResponse, batchSize),
		batchIsFull:  make(chan bool),
	}
}

func (b *Batcher) Add(event nuclio.Event, responseChan *common.ChannelWithRecover) {
	b.currentBatch <- &BatchedEventWithResponse{event: event, responseChan: responseChan}

	// if batchIsFull, Write to `batchIsFull` chan, so that we send batch to worker right when batch len reached the maximum
	if cap(b.currentBatch) == len(b.currentBatch) {
		b.batchIsFull <- true
	}
}

func (b *Batcher) WaitForBatch(batchTimeout time.Duration) ([]nuclio.Event, map[string]*common.ChannelWithRecover) {
	for {
		if b.batchIsEmpty() {
			continue
		}
		select {
		case <-b.batchIsFull:
			return b.extractBatch()
		case <-time.After(batchTimeout):
			return b.extractBatch()
		}
	}
}

func (b *Batcher) batchIsEmpty() bool {
	return len(b.currentBatch) == 0
}

func (b *Batcher) extractBatch() ([]nuclio.Event, map[string]*common.ChannelWithRecover) {

	batchLength := len(b.currentBatch)
	responseChans := make(map[string]*common.ChannelWithRecover)
	batch := make([]nuclio.Event, batchLength)

	for i := 0; i < batchLength; i++ {
		batchedEventWithResponse := <-b.currentBatch
		batch[i] = batchedEventWithResponse.event
		eventId := batchedEventWithResponse.event.GetID()
		if eventId == "" {
			eventId = nuclio.ID(uuid.New().String())
			batchedEventWithResponse.event.SetID(eventId)
		}
		responseChans[string(eventId)] = batchedEventWithResponse.responseChan
	}
	return batch, responseChans
}
