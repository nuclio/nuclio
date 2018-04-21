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

package worker

import (
	"github.com/nuclio/nuclio/pkg/processor/cloudevent"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/status"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
)

// Worker holds all the required state and context to handle a single request
type Worker struct {
	logger               logger.Logger
	context              nuclio.Context
	index                int
	runtime              runtime.Runtime
	statistics           Statistics
	structuredCloudEvent cloudevent.Structured
	binaryCloudEvent     cloudevent.Binary
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

	// process the event at the runtime
	response, err := w.runtime.ProcessEvent(event, functionLogger)

	// check if there was a processing error. if so, log it
	if err != nil {
		w.statistics.EventsHandleError++

		// use the override function logger if passed, otherwise ask the runtime for the
		// function logger
		loggerInstance := functionLogger
		if loggerInstance == nil {
			loggerInstance = w.runtime.GetFunctionLogger()
		}

		loggerInstance.WarnWith("Function returned error", "event_id", event.GetID(), "err", err)
	} else {
		w.statistics.EventsHandleSuccess++
	}

	return response, err
}

// GetStatistics returns a pointer to the statistics object. This must not be modified by the reader
func (w *Worker) GetStatistics() *Statistics {
	return &w.statistics
}

// GetIndex returns the index of the worker, as specified during creation
func (w *Worker) GetIndex() int {
	return w.index
}

// GetIndex returns the runtime of the worker, as specified during creation
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

func (w *Worker) GetStructuredCloudEvent() *cloudevent.Structured {
	return &w.structuredCloudEvent
}

func (w *Worker) GetBinaryCloudEvent() *cloudevent.Binary {
	return &w.binaryCloudEvent
}
