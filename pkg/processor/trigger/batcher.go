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
	// plus one, because we read the first event in WaitForBatch separately
	if len(b.currentBatch)+1 == cap(b.currentBatch) {
		b.batchIsFull <- true
	}
}

func (b *Batcher) WaitForBatch(batchTimeout time.Duration) ([]nuclio.Event, map[string]*common.ChannelWithRecover) {
	for {
		// Block until the first event arrives
		firstEvent := <-b.currentBatch

		select {
		case <-b.batchIsFull:
			return b.extractBatch(firstEvent)
		case <-time.After(batchTimeout):
			return b.extractBatch(firstEvent)
		}
	}
}

func (b *Batcher) extractBatch(firstEvent *BatchedEventWithResponse) ([]nuclio.Event, map[string]*common.ChannelWithRecover) {

	// +1 because we already read the first event
	batchLength := len(b.currentBatch) + 1
	responseChans := make(map[string]*common.ChannelWithRecover)
	batch := make([]nuclio.Event, batchLength)

	// Add the first event
	eventID := firstEvent.event.GetID()
	if eventID == "" {
		eventID = nuclio.ID(uuid.New().String())
		firstEvent.event.SetID(eventID)
	}

	batch[0] = firstEvent.event
	responseChans[string(eventID)] = firstEvent.responseChan

	for i := 1; i < batchLength; i++ {
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
