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
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
)

type Statistics struct {
	receivedEvents uint64
	eventHandleSuccess uint64
	eventHandleError uint64
}

type Worker struct {
	logger  nuclio.Logger
	context nuclio.Context
	index   int
	runtime runtime.Runtime
	statistics Statistics
}

func NewWorker(parentLogger nuclio.Logger,
	index int,
	runtime runtime.Runtime) (*Worker, error) {

	newWorker := Worker{
		logger:  parentLogger,
		index:   index,
		runtime: runtime,
		context: nuclio.Context{
			Logger: parentLogger.GetChild("event").(nuclio.Logger),
		},
	}

	// return an instance of the default worker
	return &newWorker, nil
}

// called by event sources
func (w *Worker) ProcessEvent(event nuclio.Event, functionLogger nuclio.Logger) (interface{}, error) {
	event.SetID(nuclio.NewID())

	w.statistics.receivedEvents++

	// process the event at the runtime
	response, err := w.runtime.ProcessEvent(event, functionLogger)

	// check if there was a processing error. if so, log it
	if err != nil {
		w.statistics.eventHandleError++

		// use the override function logger if passed, otherwise ask the runtime for the
		// function logger
		logger := functionLogger
		if logger == nil {
			logger = w.runtime.GetFunctionLogger()
		}

		logger.WarnWith("Function returned error", "event_id", event.GetID(), "err", err)
	} else {
		w.statistics.eventHandleSuccess++
	}

	return response, err
}
