package event_source

import (
	"time"

	"github.com/nuclio/nuclio/cmd/processor/app/event"
	"github.com/nuclio/nuclio/cmd/processor/app/worker"
	"github.com/nuclio/nuclio/pkg/logger"
)

type Checkpoint *string

type EventSource interface {

	// start creating events from a given checkpoint (nil - no checkpoint)
	Start(checkpoint Checkpoint) error

	// stop creating events. returns the current checkpoint
	Stop(force bool) (Checkpoint, error)

	// get the class of source (sync, async, etc)
	GetClass() string

	// get specific kind of source (http, rabbit mq, etc)
	GetKind() string
}

type AbstractEventSource struct {
	Logger          logger.Logger
	WorkerAllocator worker.WorkerAllocator
	Class           string
	Kind            string
}

func (des *AbstractEventSource) GetClass() string {
	return des.Class
}

func (des *AbstractEventSource) GetKind() string {
	return des.Kind
}

func (des *AbstractEventSource) SubmitEventToWorker(event event.Event, timeout time.Duration) (interface{}, error, error) {

	// set event source info provider (ourselves)
	event.SetSourceProvider(des)

	// allocate a worker
	workerInstance, err := des.WorkerAllocator.Allocate(timeout)
	if err != nil {
		return nil, des.Logger.Report(err, "Failed to allocate worker"), nil
	}

	// release worker when we're done
	defer des.WorkerAllocator.Release(workerInstance)

	response, err := workerInstance.ProcessEvent(event)
	if err != nil {
		return nil, des.Logger.Report(err, "Failed to process event"), nil
	}

	return response, nil, nil
}
