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

type Worker struct {
	logger  nuclio.Logger
	context nuclio.Context
	index   int
	runtime runtime.Runtime
}

func NewWorker(parentLogger nuclio.Logger,
	index int,
	runtime runtime.Runtime) *Worker {

	newWorker := Worker{
		logger:  parentLogger,
		index:   index,
		runtime: runtime,
		context: nuclio.Context{
			Logger: parentLogger.GetChild("event").(nuclio.Logger),
		},
	}

	// return an instance of the default worker
	return &newWorker
}

// called by event sources
func (w *Worker) ProcessEvent(evt nuclio.Event) (interface{}, error) {
	evt.SetID(nuclio.NewID())

	// process the event at the runtime
	response, err := w.runtime.ProcessEvent(evt)

	return response, err
}
