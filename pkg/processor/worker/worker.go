package worker

import (
	"github.com/nuclio/nuclio/pkg/logger"
	"github.com/nuclio/nuclio/pkg/processor/event"
	"github.com/nuclio/nuclio/pkg/processor/runtime"

	"github.com/satori/go.uuid"
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
func (w *Worker) ProcessEvent(event event.Event) (interface{}, error) {

	// create a unique request ID
	id := uuid.NewV4()
	event.SetID(&id)

	// process the event at the runtime
	response, err := w.runtime.ProcessEvent(event)

	// update basic statistics
	if err != nil {
		w.statistics.Failed++
	} else {
		w.statistics.Succeeded++
	}

	return response, err
}
