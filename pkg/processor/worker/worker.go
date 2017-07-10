package worker

import (
	"github.com/nuclio/nuclio-sdk/event"
	"github.com/nuclio/nuclio-sdk/logger"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
)

type Worker struct {
	logger     logger.Logger
	statistics Statistics
	context    event.Context
	index      int
	runtime    runtime.Runtime
}

func NewWorker(parentLogger logger.Logger,
	index int,
	runtime runtime.Runtime) *Worker {

	newWorker := Worker{
		logger:  parentLogger,
		index:   index,
		runtime: runtime,
		context: event.Context{
			Logger: parentLogger.GetChild("event").(logger.Logger),
		},
	}

	// return an instance of the default worker
	return &newWorker
}

// called by event sources
func (w *Worker) ProcessEvent(evt event.Event) (interface{}, error) {

	evt.SetID(event.NewID())

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
