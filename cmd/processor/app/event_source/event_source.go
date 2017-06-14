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

func (aes *AbstractEventSource) GetClass() string {
	return aes.Class
}

func (aes *AbstractEventSource) GetKind() string {
	return aes.Kind
}

func (aes *AbstractEventSource) SubmitEventToWorker(event event.Event, timeout time.Duration) (interface{}, error, error) {

	// set event source info provider (ourselves)
	event.SetSourceProvider(aes)

	// allocate a worker
	workerInstance, err := aes.WorkerAllocator.Allocate(timeout)
	if err != nil {
		return nil, aes.Logger.Report(err, "Failed to allocate worker"), nil
	}

	// release worker when we're done
	defer aes.WorkerAllocator.Release(workerInstance)

	response, err := workerInstance.ProcessEvent(event)
	if err != nil {
		return nil, aes.Logger.Report(err, "Failed to process event"), nil
	}

	return response, nil, nil
}
