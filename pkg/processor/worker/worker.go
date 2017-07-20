package worker

import (
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
)

type Worker struct {
	logger     nuclio.Logger
	statistics Statistics
	context    nuclio.Context
	index      int
	runtime    runtime.Runtime
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

	// update basic statistics
	if err != nil {
		w.statistics.Failed++
	} else {
		w.statistics.Succeeded++
	}

	return response, err
}
