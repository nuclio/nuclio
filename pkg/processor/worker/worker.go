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

package worker

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/processor/cloudevent"
	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/util/clock"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

// Worker holds all the required state and context to handle a single request
type Worker struct {

	// accessed atomically, keep as first field for alignment
	statistics Statistics

	logger               logger.Logger
	index                int
	runtime              runtime.Runtime
	structuredCloudEvent cloudevent.Structured
	binaryCloudEvent     cloudevent.Binary
	eventTime            *time.Time
}

// NewWorker creates a new worker
func NewWorker(parentLogger logger.Logger,
	index int,
	runtime runtime.Runtime) (*Worker, error) {

	newWorker := Worker{
		logger:  parentLogger,
		index:   index,
		runtime: runtime,
	}

	// return an instance of the default worker
	return &newWorker, nil
}

// ProcessEvent sends the event to the associated runtime
func (w *Worker) ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (interface{}, error) {
	w.eventTime = clock.Now()

	// process the event at the runtime
	response, err := w.runtime.ProcessEvent(event, functionLogger)
	w.eventTime = nil

	// check if there was a processing error. if so, log it
	if err != nil {
		atomic.AddUint64(&w.statistics.EventsHandledError, 1)
	} else {
		success := true

		switch typedResponse := response.(type) {
		case *nuclio.Response:
			success = typedResponse.StatusCode < http.StatusBadRequest
		case nuclio.Response:
			success = typedResponse.StatusCode < http.StatusBadRequest
		}

		if success {
			atomic.AddUint64(&w.statistics.EventsHandledSuccess, 1)
		} else {
			atomic.AddUint64(&w.statistics.EventsHandledError, 1)
		}
	}

	return response, err
}

func (w *Worker) ProcessEventBatch(batch []nuclio.Event) (interface{}, error) {
	return w.runtime.ProcessBatch(batch, w.logger)
}

// GetStatistics returns a pointer to the statistics object. This must not be modified by the reader
func (w *Worker) GetStatistics() *Statistics {
	return &w.statistics
}

// GetIndex returns the index of the worker, as specified during creation
func (w *Worker) GetIndex() int {
	return w.index
}

// GetRuntime returns the runtime of the worker, as specified during creation
func (w *Worker) GetRuntime() runtime.Runtime {
	return w.runtime
}

// GetStatus returns the status of the worker, as updated by the runtime
func (w *Worker) GetStatus() status.Status {
	return w.runtime.GetStatus()
}

// Stop stops the worker and associated runtime
func (w *Worker) Stop() error {
	return w.runtime.Stop()
}

// GetStructuredCloudEvent return a structued clould event
func (w *Worker) GetStructuredCloudEvent() *cloudevent.Structured {
	return &w.structuredCloudEvent
}

// GetBinaryCloudEvent return a binary cloud event
func (w *Worker) GetBinaryCloudEvent() *cloudevent.Binary {
	return &w.binaryCloudEvent
}

// GetEventTime return current event time, nil if we're not handling event
func (w *Worker) GetEventTime() *time.Time {
	return w.eventTime
}

// ResetEventTime resets the event time
func (w *Worker) ResetEventTime() {
	w.eventTime = nil
}

// Restart restarts the worker
func (w *Worker) Restart() error {
	w.eventTime = nil
	return w.runtime.Restart()
}

// SupportsRestart returns true if the underlying runtime supports restart
func (w *Worker) SupportsRestart() bool {
	return w.runtime.SupportsRestart()
}

func (w *Worker) Terminate() error {
	if err := w.runtime.Terminate(); err != nil {
		return err
	}
	w.logger.DebugWith("Successfully terminated worker", "workerIndex", w.index)
	return nil
}

func (w *Worker) Drain() error {
	if err := w.runtime.Drain(); err != nil {
		return err
	}
	w.logger.DebugWith("Successfully drained worker", "workerIndex", w.index)
	return nil
}

func (w *Worker) Continue() error {
	if err := w.runtime.Continue(); err != nil {
		return err
	}
	w.logger.DebugWith("Successfully continued worker", "workerIndex", w.index)
	return nil
}

// Subscribe subscribes to a control message kind
func (w *Worker) Subscribe(kind controlcommunication.ControlMessageKind, channel chan *controlcommunication.ControlMessage) error {
	return w.runtime.GetControlMessageBroker().Subscribe(kind, channel)
}

// Unsubscribe unsubscribes from a control message kind
func (w *Worker) Unsubscribe(kind controlcommunication.ControlMessageKind, channel chan *controlcommunication.ControlMessage) error {
	return w.runtime.GetControlMessageBroker().Unsubscribe(kind, channel)
}
