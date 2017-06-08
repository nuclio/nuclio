package worker

import (
	"sync"

	"github.com/satori/go.uuid"

	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/runtime"
	"github.com/nuclio/nuclio/pkg/logger"
)

var (
	wLock   sync.Mutex
	workers = make(map[int]*Worker)
	nextID  = 0
)

type Worker struct {
	logger     logger.Logger
	statistics Statistics
	context    event.Context
	index      int
	runtime    runtime.Runtime
	id         int
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

	wLock.Lock()
	defer wLock.Unlock()

	newWorker.id = nextID
	nextID++
	workers[newWorker.id] = &newWorker

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

func (worker *Worker) Context() *event.Context {
	return &worker.context
}

func (worker *Worker) Statistics() *Statistics {
	return &worker.statistics
}

func AllWorkers() map[int]*Worker {
	return workers
}

func FindWorker(id int) *Worker {
	wLock.Lock()
	defer wLock.Unlock()

	return workers[id]
}
