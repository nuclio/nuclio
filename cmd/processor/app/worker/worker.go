package worker

import (
	"github.com/satori/go.uuid"

	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/pkg/logger"
	"github.com/nuclio/nuclio/cmd/processor/app/runtime"
)

type Worker struct {
	logger     logger.Logger
	statistics Statistics
	context    event.Context
	index      int
	runtime    runtime.Runtime
}

func NewWorker(logger logger.Logger,
	index int,
	runtime runtime.Runtime) *Worker {

	newWorker := Worker{
		logger:  logger,
		index:   index,
		runtime: runtime,
		context: event.Context{
			Logger: logger.GetChild("event"),
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
