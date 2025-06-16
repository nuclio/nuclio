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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/cloudevent"
	"github.com/nuclio/nuclio/pkg/processor/controlcommunication"
	"github.com/nuclio/nuclio/pkg/processor/eventprocessor"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/statistics"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

// Worker holds all the required state and context to handle a single request
type Worker struct {

	// accessed atomically, keep as first field for alignment
	statistics statistics.EventProcessingStatistics

	logger               logger.Logger
	index                int
	runtime              runtime.Runtime
	structuredCloudEvent cloudevent.Structured
	binaryCloudEvent     cloudevent.Binary
}

// NewWorker creates a new worker
func NewWorker(parentLogger logger.Logger,
	index int,
	runtime runtime.Runtime) (eventprocessor.EventProcessor, error) {

	newWorker := Worker{
		logger:  parentLogger,
		index:   index,
		runtime: runtime,
	}

	// return an instance of the default worker
	return &newWorker, nil
}

// ProcessEvent sends the event to the associated runtime
func (w *Worker) ProcessEvent(event nuclio.Event, functionLogger logger.Logger) (nuclio.ProcessingResult, error) {
	// process the event at the runtime
	response, err := w.runtime.ProcessEvent(event, functionLogger)

	w.calculateProcessingMetrics(response, err)

	if response == nil {
		// if the response is nil, return an empty response with the error
		// it might be that go runtime handler returned nil, so we want to make sure that we return a valid response
		return &nuclio.Response{}, err
	}

	// always translate the response to a nuclio.ProcessingResult
	switch typedResponse := response.(type) {
	case *nuclio.Response:
		return typedResponse, err
	case nuclio.Response:
		return &typedResponse, err
	case *nuclio.ResponseStream:
		return typedResponse, err
	case nuclio.ResponseStream:
		return &typedResponse, err
	case io.ReadCloser:
		// if the response is an io.ReadCloser, create a response stream
		return nuclio.NewCustomResponseStream("", nil, 0, typedResponse, nil), err
	case []byte:
		return &nuclio.Response{
			Body: typedResponse,
		}, err
	case string:
		return &nuclio.Response{
			Body: []byte(typedResponse),
		}, err
	default:
		// try to JSON-marshal the value
		if marshaled, marshalErr := json.Marshal(typedResponse); marshalErr == nil {
			return &nuclio.Response{Body: marshaled}, err
		}
		// fallback to string formatting if JSON marshalling fails
		return &nuclio.Response{Body: []byte(fmt.Sprintf("%v", typedResponse))}, err
	}
}

func (w *Worker) ProcessEventBatch(batch []nuclio.Event, functionLogger logger.Logger) ([]*runtime.ResponseWithErrors, error) {
	return w.runtime.ProcessBatch(batch, w.logger)
}

// GetStatistics returns a pointer to the statistics object. This must not be modified by the reader
func (w *Worker) GetStatistics() *statistics.EventProcessingStatistics {
	return &w.statistics
}

func (w *Worker) GetAllocationStatistics() *statistics.AllocatorStatistics {
	return w.runtime.GetAllocationStatistics()
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

// SetStatus sets worker status
func (w *Worker) SetStatus(newStatus status.Status) {
	w.runtime.SetStatus(newStatus)
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

// Restart restarts the worker
func (w *Worker) Restart() error {
	return w.runtime.Restart()
}

// SupportsRestart returns true if the underlying runtime supports restart
func (w *Worker) SupportsRestart() bool {
	return w.runtime.SupportsRestart()
}

// RestartRequired returns whether the worker requires a restart
func (w *Worker) RestartRequired() bool {
	return w.runtime.RestartRequired()
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

func (w *Worker) WaitForStart(time.Duration) error {
	return nil
}

func (w *Worker) RunHandler() {
}

func (w *Worker) IsAsync() bool {
	return w.runtime.GetConfiguration().Mode == functionconfig.AsyncTriggerWorkMode
}

func (w *Worker) IsBusy() bool {
	return w.runtime.IsBusy()
}

func (w *Worker) calculateProcessingMetrics(response interface{}, err error) {
	// check if there was a processing error. if so, log it
	if err != nil {
		atomic.AddUint64(&w.statistics.EventsHandledError, 1)
		return
	}
	success := true
	stream := false

	if typedResponse, ok := response.(nuclio.ProcessingResult); ok {
		if typedResponse.GetStatusCode() > 0 {
			success = typedResponse.GetStatusCode() < http.StatusBadRequest
		}
		stream = typedResponse.IsStream()
	}

	if stream {
		if success {
			atomic.AddUint64(&w.statistics.EventsStreamingStartedSuccessfully, 1)
		} else {
			atomic.AddUint64(&w.statistics.EventsStreamingStartedError, 1)
		}
		return
	}

	if success {
		atomic.AddUint64(&w.statistics.EventsHandledSuccess, 1)
	} else {
		atomic.AddUint64(&w.statistics.EventsHandledError, 1)
	}

}
